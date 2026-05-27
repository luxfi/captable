// Per-state fee schedule for Rule 506 notice filings.
//
// Authoritative source: each state's securities-administrator
// regulation + the NASAA EFD "Schedule of State Fees (Form D)" at
// http://nasaaefd.org/About/FormDStates.
//
// The schedule below captures the standard Rule 506(b) / 506(c)
// notice-of-sale fee per state as of public publication. Fees that
// scale with offering size carry an explicit Notes line.
//
// Source-of-design: Public-Spec
// Source-ref: http://nasaaefd.org/About/FormDStates

package bluesky

// stateFee is the per-state fee + notes pair returned by
// stateFeeSchedule.
type stateFee struct {
	state float64
	notes string
}

// stateFeeSchedule returns the per-state fee for a notice-of-sale
// filing under the supplied notice. Values are the published Rule 506
// notice fees per state; some states scale the fee with offering size.
func stateFeeSchedule(state State, n *NoticeFiling) stateFee {
	amt := 0.0
	if n != nil {
		amt = n.Offering.TotalOfferingAmount
	}
	switch state {
	// Flat-fee states (most common).
	case "AL":
		return stateFee{300, "Alabama: $300 per filing; $300 lifetime cap"}
	case "AK":
		return stateFee{600, "Alaska: $600 per filing"}
	case "AZ":
		return stateFee{250, "Arizona: $250 per filing"}
	case "AR":
		return stateFee{100, "Arkansas: $100 per filing; $500 lifetime cap"}
	case "CA":
		// CA scales: $25 minimum + 0.001% of offering (max $300).
		fee := 25.0 + 0.00001*amt
		if fee > 300 {
			fee = 300
		}
		return stateFee{fee, "California: $25 + 0.001% of offering (max $300)"}
	case "CO":
		return stateFee{75, "Colorado: $75 per filing"}
	case "CT":
		return stateFee{150, "Connecticut: $150 per filing"}
	case "DE":
		return stateFee{200, "Delaware: $200 per filing"}
	case "DC":
		return stateFee{250, "DC: $250 per filing"}
	case "FL":
		return stateFee{200, "Florida: $200 paper filing (no EFD)"}
	case "GA":
		return stateFee{250, "Georgia: $250 per filing"}
	case "HI":
		return stateFee{200, "Hawaii: $200 per filing"}
	case "ID":
		return stateFee{50, "Idaho: $50 per filing"}
	case "IL":
		return stateFee{200, "Illinois: $200 per filing"}
	case "IN":
		return stateFee{100, "Indiana: $100 per filing"}
	case "IA":
		return stateFee{100, "Iowa: $100 per filing"}
	case "KS":
		return stateFee{250, "Kansas: $250 per filing"}
	case "KY":
		return stateFee{250, "Kentucky: $250 per filing"}
	case "LA":
		return stateFee{300, "Louisiana: $300 per filing"}
	case "ME":
		return stateFee{300, "Maine: $300 per filing"}
	case "MD":
		return stateFee{100, "Maryland: $100 per filing"}
	case "MA":
		return stateFee{300, "Massachusetts: $300 per filing; $750 lifetime cap"}
	case "MI":
		return stateFee{100, "Michigan: $100 per filing; $100 lifetime cap"}
	case "MN":
		return stateFee{50, "Minnesota: $50 per filing; $300 lifetime cap"}
	case "MS":
		return stateFee{300, "Mississippi: $300 per filing"}
	case "MO":
		return stateFee{100, "Missouri: $100 per filing"}
	case "MT":
		return stateFee{200, "Montana: $200 per filing"}
	case "NE":
		return stateFee{200, "Nebraska: $200 per filing"}
	case "NV":
		return stateFee{350, "Nevada: $350 per filing"}
	case "NH":
		return stateFee{500, "New Hampshire: $500 per filing"}
	case "NJ":
		return stateFee{250, "New Jersey: $250 per filing"}
	case "NM":
		return stateFee{350, "New Mexico: $350 per filing"}
	case "NY":
		// NY scales by offering: $300 if ≤ $500K; $1,200 if > $500K.
		if amt > 500_000 {
			return stateFee{1200, "New York: $1,200 (offerings > $500K)"}
		}
		return stateFee{300, "New York: $300 (offerings ≤ $500K)"}
	case "NC":
		return stateFee{350, "North Carolina: $350 per filing"}
	case "ND":
		return stateFee{100, "North Dakota: $100 per filing"}
	case "OH":
		return stateFee{100, "Ohio: $100 per filing"}
	case "OK":
		return stateFee{100, "Oklahoma: $100 per filing"}
	case "OR":
		return stateFee{225, "Oregon: $225 per filing"}
	case "PA":
		return stateFee{300, "Pennsylvania: $300 per filing"}
	case "RI":
		return stateFee{300, "Rhode Island: $300 per filing; $300 lifetime cap"}
	case "SC":
		return stateFee{300, "South Carolina: $300 per filing"}
	case "SD":
		return stateFee{150, "South Dakota: $150 per filing"}
	case "TN":
		return stateFee{500, "Tennessee: $500 per filing"}
	case "TX":
		return stateFee{500, "Texas: $500 paper filing (no EFD); $500 lifetime cap"}
	case "UT":
		return stateFee{60, "Utah: $60 per filing"}
	case "VT":
		return stateFee{600, "Vermont: $600 per filing"}
	case "VA":
		return stateFee{250, "Virginia: $250 per filing"}
	case "WA":
		// WA scales: $300 minimum, 0.1% on amount > $50K, max $1,500.
		fee := 300.0
		if amt > 50_000 {
			scaled := 300.0 + 0.001*(amt-50_000)
			if scaled > 1500 {
				scaled = 1500
			}
			if scaled > fee {
				fee = scaled
			}
		}
		return stateFee{fee, "Washington: $300 + 0.1% on amount > $50K (max $1,500)"}
	case "WV":
		return stateFee{250, "West Virginia: $250 per filing"}
	case "WI":
		return stateFee{200, "Wisconsin: $200 per filing"}
	case "WY":
		return stateFee{200, "Wyoming: $200 per filing"}
	}
	return stateFee{0, "unknown state — verify fee schedule before filing"}
}
