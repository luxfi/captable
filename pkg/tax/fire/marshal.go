// FIRE fixed-width record marshaling per Publication 1220 Part C.
//
// Every record is exactly RecordLen (750) bytes plus CR-LF; numeric
// fields are right-justified zero-filled, alphanumeric fields are
// left-justified blank-filled. Dollar amounts are in cents (no decimal
// point) and take exactly 12 positions, right-justified zero-filled.
//
// Source-of-design: Public-Spec
// Source-ref: IRS Publication 1220 Part C — Record Formats
package fire

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// jsonUnmarshal is a small wrapper to keep the import set in
// client.go small.
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// recordBuf is a builder for a single fixed-width record. The buffer
// is pre-allocated to RecordLen bytes of blanks and overwritten
// positionally; on Bytes() it is returned with a trailing CR-LF.
type recordBuf struct {
	buf []byte
}

// newRecord returns a recordBuf of RecordLen blank bytes.
func newRecord() *recordBuf {
	b := make([]byte, RecordLen)
	for i := range b {
		b[i] = ' '
	}
	return &recordBuf{buf: b}
}

// putChar writes one byte at one-based column col (column-1 is the
// record-type marker). Out-of-range writes are silently dropped to
// keep the marshal side panic-free.
func (r *recordBuf) putChar(col int, c byte) {
	idx := col - 1
	if idx < 0 || idx >= len(r.buf) {
		return
	}
	r.buf[idx] = c
}

// putString writes s into the [start, end] one-based inclusive range.
// Strings are left-justified, blank-padded; longer strings are
// truncated.
func (r *recordBuf) putString(start, end int, s string) {
	a := start - 1
	b := end // inclusive
	if a < 0 || b > len(r.buf) || a >= b {
		return
	}
	width := b - a
	if len(s) > width {
		s = s[:width]
	}
	copy(r.buf[a:a+len(s)], s)
}

// putUpper writes s upper-cased into the range.
func (r *recordBuf) putUpper(start, end int, s string) {
	r.putString(start, end, strings.ToUpper(s))
}

// putAlnum writes s into the range stripped of non-alnum characters,
// upper-cased.
func (r *recordBuf) putAlnum(start, end int, s string) {
	var b strings.Builder
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		}
	}
	r.putUpper(start, end, b.String())
}

// putDigits writes a numeric string into the range, right-justified
// zero-padded. Non-digit characters are stripped.
func (r *recordBuf) putDigits(start, end int, s string) {
	a := start - 1
	b := end
	if a < 0 || b > len(r.buf) || a >= b {
		return
	}
	width := b - a
	digits := stripNonDigits(s)
	if len(digits) > width {
		digits = digits[len(digits)-width:]
	}
	// Right-justify zero-padded.
	for i := a; i < a+(width-len(digits)); i++ {
		r.buf[i] = '0'
	}
	copy(r.buf[a+width-len(digits):a+width], digits)
}

// putIntZeroPad writes an integer right-justified zero-padded.
func (r *recordBuf) putIntZeroPad(start, end int, v int) {
	r.putDigits(start, end, strconv.Itoa(v))
}

// putCents writes a dollar amount (in cents) right-justified zero-
// padded into a 12-position field by default.
func (r *recordBuf) putCents(start, end int, cents int64) {
	r.putDigits(start, end, strconv.FormatInt(cents, 10))
}

// putBool writes '1' / blank for true / false at one column.
func (r *recordBuf) putBool(col int, v bool) {
	if v {
		r.putChar(col, '1')
	}
}

// putBoolMark writes a specific mark char if v is true; blank
// otherwise. Used for FIRE's overloaded "X" / "T" / "P" / "G"
// indicators.
func (r *recordBuf) putBoolMark(col int, v bool, mark byte) {
	if v {
		r.putChar(col, mark)
	}
}

// bytes returns the record followed by CR-LF.
func (r *recordBuf) bytes() []byte {
	out := make([]byte, 0, len(r.buf)+2)
	out = append(out, r.buf...)
	out = append(out, '\r', '\n')
	return out
}

// stripNonDigits returns s with all non-digit characters removed.
func stripNonDigits(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// amountColRange returns the (start, end) one-based column range for
// the given FIRE amount code on a B record per Publication 1220 Part
// C §2 (B record layout). Amount codes are '1' through '9' followed by
// 'A' through 'I' (18 codes), each occupying 12 positions starting at
// column 55.
func amountColRange(code byte) (int, int, bool) {
	var index int
	switch {
	case code >= '1' && code <= '9':
		index = int(code - '1')
	case code >= 'A' && code <= 'I':
		index = 9 + int(code-'A')
	default:
		return 0, 0, false
	}
	start := 55 + index*12
	end := start + 11
	return start, end, true
}

// amountTotalsRange returns the (start, end) one-based column range
// for the given FIRE amount code on a C or K record. Totals are 18-
// position fields, 18 codes total, starting at column 16 on a C
// record.
func amountTotalsRange(code byte, baseCol int) (int, int, bool) {
	var index int
	switch {
	case code >= '1' && code <= '9':
		index = int(code - '1')
	case code >= 'A' && code <= 'I':
		index = 9 + int(code-'A')
	default:
		return 0, 0, false
	}
	start := baseCol + index*18
	end := start + 17
	return start, end, true
}

// Marshal serializes a FIREFile into the FIRE fixed-width wire
// format. The output is the concatenation of T + (A + B* + C)* + K* +
// F records, each terminated with CR-LF.
func Marshal(f *FIREFile) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("nil file")
	}
	var buf bytes.Buffer

	// Populate sequence numbers and counters as we render. The T
	// record is always sequence 1.
	seq := 1
	if f.Transmitter.SequenceNum == 0 {
		f.Transmitter.SequenceNum = seq
	}
	seq++

	tRec, err := marshalTransmitter(&f.Transmitter)
	if err != nil {
		return nil, fmt.Errorf("transmitter record: %w", err)
	}
	buf.Write(tRec)

	totalPayers := 0
	totalPayees := 0

	for i := range f.PayerGroups {
		grp := &f.PayerGroups[i]
		if grp.Payer.SequenceNum == 0 {
			grp.Payer.SequenceNum = seq
		}
		seq++
		if grp.Payer.PaymentYear == 0 {
			grp.Payer.PaymentYear = f.Transmitter.PaymentYear
		}

		aRec, err := marshalPayer(&grp.Payer)
		if err != nil {
			return nil, fmt.Errorf("payer group %d: %w", i, err)
		}
		buf.Write(aRec)
		totalPayers++

		// Accumulate per-amount-code totals for the C record.
		totals := make(map[byte]int64)
		for j := range grp.Payees {
			pe := &grp.Payees[j]
			if pe.SequenceNum == 0 {
				pe.SequenceNum = seq
			}
			seq++
			if pe.PaymentYear == 0 {
				pe.PaymentYear = grp.Payer.PaymentYear
			}
			bRec, err := marshalPayee(pe, grp.Payer.AmountCodes)
			if err != nil {
				return nil, fmt.Errorf("payee %d in group %d: %w", j, i, err)
			}
			buf.Write(bRec)
			for c, amt := range pe.PaymentAmounts {
				totals[c] += amt
			}
			totalPayees++
		}
		if grp.EndOfPayer.SequenceNum == 0 {
			grp.EndOfPayer.SequenceNum = seq
		}
		seq++
		if grp.EndOfPayer.NumPayees == 0 {
			grp.EndOfPayer.NumPayees = len(grp.Payees)
		}
		if grp.EndOfPayer.AmountTotals == nil {
			grp.EndOfPayer.AmountTotals = totals
		}
		cRec, err := marshalEndOfPayer(&grp.EndOfPayer)
		if err != nil {
			return nil, fmt.Errorf("end-of-payer group %d: %w", i, err)
		}
		buf.Write(cRec)
	}

	for i := range f.StateTotals {
		st := &f.StateTotals[i]
		if st.SequenceNum == 0 {
			st.SequenceNum = seq
		}
		seq++
		kRec, err := marshalStateTotals(st)
		if err != nil {
			return nil, fmt.Errorf("state totals %d: %w", i, err)
		}
		buf.Write(kRec)
	}

	if f.EndOfFile.SequenceNum == 0 {
		f.EndOfFile.SequenceNum = seq
	}
	if f.EndOfFile.NumPayers == 0 {
		f.EndOfFile.NumPayers = totalPayers
	}
	if f.EndOfFile.NumPayees == 0 {
		f.EndOfFile.NumPayees = totalPayees
	}
	if f.Transmitter.TotalPayees == 0 {
		f.Transmitter.TotalPayees = totalPayees
	}
	// Re-render the T record with the updated TotalPayees count.
	tRec2, err := marshalTransmitter(&f.Transmitter)
	if err != nil {
		return nil, fmt.Errorf("transmitter record (rebuild): %w", err)
	}
	// Replace the first record in the buffer (rebuild from scratch).
	out := bytes.NewBuffer(make([]byte, 0, buf.Len()))
	out.Write(tRec2)
	// Append everything after the original T record (skip RecordLen+2
	// bytes for the original T + CR-LF).
	remainder := buf.Bytes()[RecordLen+2:]
	out.Write(remainder)

	fRec, err := marshalEndOfFile(&f.EndOfFile)
	if err != nil {
		return nil, fmt.Errorf("end-of-file: %w", err)
	}
	out.Write(fRec)
	return out.Bytes(), nil
}

// marshalTransmitter renders a T record.
func marshalTransmitter(t *TransmitterRecord) ([]byte, error) {
	r := newRecord()
	r.putChar(1, 'T')
	r.putIntZeroPad(2, 5, t.PaymentYear)
	if t.PriorYear {
		r.putChar(6, 'P')
	}
	r.putDigits(7, 15, t.TIN)
	r.putString(16, 20, padOrTrim(t.TCC, 5))
	// Position 21-27 blank.
	if t.TestFileInd {
		r.putChar(28, 'T')
	}
	if t.ForeignEntity {
		r.putChar(29, '1')
	}
	r.putString(30, 69, t.TransmitterName)
	r.putString(70, 109, t.TransmitterNameCont)
	r.putString(110, 149, t.CompanyName)
	r.putString(150, 189, t.CompanyNameCont)
	r.putString(190, 229, t.CompanyMailingAddress)
	r.putString(230, 269, t.CompanyCity)
	r.putUpper(270, 271, t.CompanyState)
	r.putDigits(272, 280, t.CompanyZip)
	// Position 281-295 blank.
	r.putIntZeroPad(296, 303, t.TotalPayees)
	r.putString(304, 343, t.ContactName)
	r.putString(344, 358, t.ContactPhone)
	r.putString(359, 408, t.ContactEmail)
	// Position 409-499 blank.
	r.putIntZeroPad(500, 507, t.SequenceNum)
	// Position 508-517 blank, 518 reserved.
	r.putString(519, 519, t.VendorIndicator)
	r.putString(520, 559, t.VendorName)
	r.putString(560, 599, t.VendorMailingAddress)
	r.putString(600, 639, t.VendorCity)
	r.putUpper(640, 641, t.VendorState)
	r.putDigits(642, 650, t.VendorZip)
	r.putString(651, 690, t.VendorContactName)
	r.putString(691, 705, t.VendorContactPhone)
	// Position 706-739 blank.
	if t.VendorForeignEntityInd {
		r.putChar(740, '1')
	}
	// Position 741-748 reserved, 749-750 blank.
	return r.bytes(), nil
}

// marshalPayer renders an A record.
func marshalPayer(p *PayerRecord) ([]byte, error) {
	r := newRecord()
	r.putChar(1, 'A')
	r.putIntZeroPad(2, 5, p.PaymentYear)
	if p.CFSFInd {
		r.putChar(6, '1')
	}
	// Position 7-11 blank.
	r.putDigits(12, 20, p.PayerTIN)
	r.putAlnum(21, 24, p.PayerNameControl)
	if p.LastFilingInd {
		r.putChar(25, '1')
	}
	if p.TypeOfReturn == "" {
		return nil, ErrUnsupportedFormCode
	}
	r.putString(26, 27, string(p.TypeOfReturn))
	r.putString(28, 45, p.AmountCodes)
	// Position 46-51 blank.
	r.putString(52, 91, p.PayerName)
	r.putString(92, 131, p.PayerNameCont)
	r.putString(132, 171, p.PayerShippingAddress)
	r.putString(172, 211, p.PayerCity)
	r.putUpper(212, 213, p.PayerState)
	r.putDigits(214, 222, p.PayerZip)
	r.putString(223, 237, p.PayerPhone)
	// Position 238-499 blank / reserved.
	r.putIntZeroPad(500, 507, p.SequenceNum)
	return r.bytes(), nil
}

// marshalPayee renders a B record. The payer's AmountCodes string
// determines which amount-code positions are populated; unused
// positions are zero-padded by the field layout.
func marshalPayee(p *PayeeRecord, amountCodes string) ([]byte, error) {
	r := newRecord()
	r.putChar(1, 'B')
	r.putIntZeroPad(2, 5, p.PaymentYear)
	r.putString(6, 6, p.CorrectedReturnInd)
	r.putAlnum(7, 10, p.NameControl)
	r.putString(11, 11, p.TypeOfTIN)
	r.putDigits(12, 20, p.PayeeTIN)
	r.putString(21, 40, p.PayerAccountNum)
	r.putString(41, 44, p.PayerOfficeCode)
	// Position 45-54 blank/reserved.
	// Amounts at codes 1-9 / A-I.
	for _, code := range amountCodes {
		c := byte(code)
		start, end, ok := amountColRange(c)
		if !ok {
			continue
		}
		amt := p.PaymentAmounts[c]
		r.putCents(start, end, amt)
	}
	if p.ForeignCountryInd {
		r.putChar(247, '1')
	}
	r.putString(248, 287, p.PayeeFirstNameLine)
	r.putString(288, 327, p.PayeeSecondNameLine)
	r.putString(367, 406, p.PayeeMailingAddress)
	r.putString(407, 446, p.PayeeCity)
	r.putUpper(447, 448, p.PayeeState)
	r.putDigits(449, 457, p.PayeeZip)
	r.putIntZeroPad(500, 507, p.SequenceNum)
	if p.SecondTINNotice {
		r.putChar(545, '2')
	}
	// Form-specific variable-position fields.
	for col, val := range p.FormSpecificFields {
		if col >= 1 && col <= RecordLen {
			r.putString(col, col+len(val)-1, val)
		}
	}
	// CFSF state info block.
	if p.StateInfo.SpecialDataEntries != "" || p.StateInfo.CombinedFedStateCode != "" ||
		p.StateInfo.StateIncomeTaxWithheld != 0 || p.StateInfo.LocalIncomeTaxWithheld != 0 {
		r.putString(663, 722, p.StateInfo.SpecialDataEntries)
		r.putCents(723, 734, p.StateInfo.StateIncomeTaxWithheld)
		r.putCents(735, 746, p.StateInfo.LocalIncomeTaxWithheld)
		r.putString(747, 748, p.StateInfo.CombinedFedStateCode)
	}
	return r.bytes(), nil
}

// marshalEndOfPayer renders a C record.
func marshalEndOfPayer(c *EndOfPayerRecord) ([]byte, error) {
	r := newRecord()
	r.putChar(1, 'C')
	r.putIntZeroPad(2, 9, c.NumPayees)
	// Position 10-15 blank.
	for code, total := range c.AmountTotals {
		start, end, ok := amountTotalsRange(code, 16)
		if !ok {
			continue
		}
		r.putCents(start, end, total)
	}
	// Position 340-499 blank / reserved.
	r.putIntZeroPad(500, 507, c.SequenceNum)
	return r.bytes(), nil
}

// marshalStateTotals renders a K record.
func marshalStateTotals(k *StateTotalsRecord) ([]byte, error) {
	r := newRecord()
	r.putChar(1, 'K')
	r.putIntZeroPad(2, 9, k.NumPayees)
	// Position 10-15 blank.
	for code, total := range k.AmountTotals {
		start, end, ok := amountTotalsRange(code, 16)
		if !ok {
			continue
		}
		r.putCents(start, end, total)
	}
	r.putIntZeroPad(500, 507, k.SequenceNum)
	r.putCents(707, 724, k.StateIncomeTaxWithheld)
	r.putCents(725, 742, k.LocalIncomeTaxWithheld)
	r.putString(747, 748, k.CombinedFedStateCode)
	return r.bytes(), nil
}

// marshalEndOfFile renders an F record.
func marshalEndOfFile(f *EndOfFileRecord) ([]byte, error) {
	r := newRecord()
	r.putChar(1, 'F')
	r.putIntZeroPad(2, 9, f.NumPayers)
	// Position 10-21 blank.
	r.putIntZeroPad(22, 29, f.NumPayees)
	// Position 30-499 blank / reserved.
	r.putIntZeroPad(500, 507, f.SequenceNum)
	return r.bytes(), nil
}

// padOrTrim normalizes s to exactly width characters by truncation or
// right-padding with blanks.
func padOrTrim(s string, width int) string {
	if len(s) > width {
		return s[:width]
	}
	if len(s) < width {
		return s + strings.Repeat(" ", width-len(s))
	}
	return s
}
