package conversion

import (
	"fmt"
	"math"
)

// ConvertSAFE computes the conversion of a SAFE into equity given a trigger event.
func ConvertSAFE(safe SAFE, trigger ConversionTrigger) (*ConversionResult, error) {
	if safe.InvestmentAmount <= 0 {
		return nil, fmt.Errorf("investment_amount must be positive")
	}
	if trigger.RoundPricePerShare <= 0 && safe.ValuationCap <= 0 {
		return nil, fmt.Errorf("either round_price_per_share or valuation_cap must be positive")
	}

	var capPrice, discountPrice float64
	var method string

	// Cap price calculation depends on SAFE type.
	if safe.ValuationCap > 0 {
		switch safe.Type {
		case "post_money":
			if trigger.PostMoneyShares <= 0 {
				return nil, fmt.Errorf("post_money_shares required for post-money SAFE conversion")
			}
			capPrice = safe.ValuationCap / float64(trigger.PostMoneyShares)
		default:
			// pre_money or mfn: use pre-money shares.
			if trigger.PreMoneyShares <= 0 {
				return nil, fmt.Errorf("pre_money_shares required for SAFE conversion")
			}
			capPrice = safe.ValuationCap / float64(trigger.PreMoneyShares)
		}
	}

	// Discount price.
	if safe.Discount > 0 && trigger.RoundPricePerShare > 0 {
		discountPrice = trigger.RoundPricePerShare * (1.0 - safe.Discount/100.0)
	}

	// Determine effective price (best for investor = lowest price).
	effectivePrice := selectBestPrice(capPrice, discountPrice, trigger.RoundPricePerShare)
	if effectivePrice <= 0 {
		return nil, fmt.Errorf("could not determine a valid conversion price")
	}

	// Determine method.
	switch {
	case capPrice > 0 && discountPrice > 0:
		if capPrice <= discountPrice {
			method = "cap"
		} else {
			method = "discount"
		}
	case capPrice > 0:
		method = "cap"
	case discountPrice > 0:
		method = "discount"
	default:
		method = "mfn"
	}

	shares := int64(math.Floor(safe.InvestmentAmount / effectivePrice))

	var effectiveDiscount float64
	if trigger.RoundPricePerShare > 0 {
		effectiveDiscount = (1.0 - effectivePrice/trigger.RoundPricePerShare) * 100.0
	}

	return &ConversionResult{
		InstrumentID:      safe.ID,
		InstrumentType:    "safe",
		InvestorID:        safe.InvestorID,
		OriginalInvestment: safe.InvestmentAmount,
		ConvertedAmount:   safe.InvestmentAmount,
		SharesIssued:      shares,
		PricePerShare:     effectivePrice,
		EffectiveDiscount: effectiveDiscount,
		ShareClassName:    "SAFE Preferred",
		Method:            method,
	}, nil
}

// ConvertNote computes the conversion of a convertible note into equity given a trigger event.
func ConvertNote(note ConvertibleNote, trigger ConversionTrigger) (*ConversionResult, error) {
	if note.PrincipalAmount <= 0 {
		return nil, fmt.Errorf("principal_amount must be positive")
	}
	if trigger.RoundPricePerShare <= 0 && note.ValuationCap <= 0 {
		return nil, fmt.Errorf("either round_price_per_share or valuation_cap must be positive")
	}

	// Calculate accrued interest.
	accruedInterest := computeAccruedInterest(note, trigger)
	convertedAmount := note.PrincipalAmount + accruedInterest

	// Cap price.
	var capPrice float64
	if note.ValuationCap > 0 {
		if trigger.PreMoneyShares <= 0 {
			return nil, fmt.Errorf("pre_money_shares required for note conversion with cap")
		}
		capPrice = note.ValuationCap / float64(trigger.PreMoneyShares)
	}

	// Discount price.
	var discountPrice float64
	if note.Discount > 0 && trigger.RoundPricePerShare > 0 {
		discountPrice = trigger.RoundPricePerShare * (1.0 - note.Discount/100.0)
	}

	// Effective price (best for investor = lowest price).
	effectivePrice := selectBestPrice(capPrice, discountPrice, trigger.RoundPricePerShare)
	if effectivePrice <= 0 {
		return nil, fmt.Errorf("could not determine a valid conversion price")
	}

	// Determine method.
	var method string
	switch {
	case capPrice > 0 && discountPrice > 0:
		if capPrice <= discountPrice {
			method = "cap"
		} else {
			method = "discount"
		}
	case capPrice > 0:
		method = "cap"
	case discountPrice > 0:
		method = "discount"
	default:
		method = "mfn"
	}

	shares := int64(math.Floor(convertedAmount / effectivePrice))

	var effectiveDiscount float64
	if trigger.RoundPricePerShare > 0 {
		effectiveDiscount = (1.0 - effectivePrice/trigger.RoundPricePerShare) * 100.0
	}

	return &ConversionResult{
		InstrumentID:      note.ID,
		InstrumentType:    "note",
		InvestorID:        note.InvestorID,
		OriginalInvestment: note.PrincipalAmount,
		ConvertedAmount:   convertedAmount,
		SharesIssued:      shares,
		PricePerShare:     effectivePrice,
		EffectiveDiscount: effectiveDiscount,
		ShareClassName:    "Note Preferred",
		Method:            method,
	}, nil
}

// ConvertAll converts multiple SAFEs and convertible notes in a single trigger event.
func ConvertAll(safes []SAFE, notes []ConvertibleNote, trigger ConversionTrigger) ([]ConversionResult, error) {
	results := make([]ConversionResult, 0, len(safes)+len(notes))

	for _, s := range safes {
		r, err := ConvertSAFE(s, trigger)
		if err != nil {
			return nil, fmt.Errorf("convert SAFE %s: %w", s.ID, err)
		}
		results = append(results, *r)
	}

	for _, n := range notes {
		r, err := ConvertNote(n, trigger)
		if err != nil {
			return nil, fmt.Errorf("convert note %s: %w", n.ID, err)
		}
		results = append(results, *r)
	}

	return results, nil
}

// selectBestPrice returns the lowest positive price among cap, discount, and round price.
// The lowest price is best for the investor (more shares per dollar).
func selectBestPrice(capPrice, discountPrice, roundPrice float64) float64 {
	best := 0.0

	candidates := []float64{capPrice, discountPrice, roundPrice}
	for _, p := range candidates {
		if p > 0 && (best == 0 || p < best) {
			best = p
		}
	}

	return best
}

// computeAccruedInterest calculates simple interest from issue date to conversion date.
func computeAccruedInterest(note ConvertibleNote, trigger ConversionTrigger) float64 {
	if note.InterestRate <= 0 {
		return 0
	}

	conversionDate := trigger.ConversionDate
	if conversionDate.IsZero() {
		conversionDate = note.MaturityDate
	}

	days := conversionDate.Sub(note.IssueDate).Hours() / 24.0
	if days <= 0 {
		return 0
	}

	return note.PrincipalAmount * (note.InterestRate / 100.0) * (days / 365.0)
}
