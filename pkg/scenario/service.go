package scenario

import "fmt"

// ModelRound computes a pro-forma cap table after a priced equity round.
// existingShares describes the current holders, round defines the new investment terms,
// and optionPoolPercent is the target option pool as a percentage of post-money fully diluted shares.
func ModelRound(existingShares []OwnershipRow, round FundingRound, optionPoolPercent float64) (*ProFormaResult, error) {
	if round.PreMoneyValuation <= 0 {
		return nil, fmt.Errorf("pre_money_valuation must be positive")
	}
	if round.InvestmentAmount <= 0 {
		return nil, fmt.Errorf("investment_amount must be positive")
	}
	if len(existingShares) == 0 {
		return nil, fmt.Errorf("existing_shares must not be empty")
	}
	if optionPoolPercent < 0 || optionPoolPercent >= 100 {
		return nil, fmt.Errorf("option_pool_percent must be in [0, 100)")
	}

	// Sum existing shares.
	var totalExisting int64
	for _, row := range existingShares {
		totalExisting += row.SharesBefore
	}
	if totalExisting <= 0 {
		return nil, fmt.Errorf("total existing shares must be positive")
	}

	// Price per share = pre-money valuation / existing fully diluted shares.
	pricePerShare := float64(round.PreMoneyValuation) / float64(totalExisting)

	// New shares issued to investor.
	newShares := int64(float64(round.InvestmentAmount) / pricePerShare)

	postMoney := round.PreMoneyValuation + round.InvestmentAmount

	// Option pool expansion: target % of post-money fully diluted.
	// Post-money fully diluted = existing + new + option pool increase.
	// optionPoolPercent/100 * (existing + new + poolIncrease) = poolIncrease
	// poolIncrease = optionPoolPercent/100 * (existing + new) / (1 - optionPoolPercent/100)
	var optionPoolIncrease int64
	var optionPoolShares int64
	if optionPoolPercent > 0 {
		pct := optionPoolPercent / 100.0
		optionPoolIncrease = int64(pct * float64(totalExisting+newShares) / (1.0 - pct))
		optionPoolShares = optionPoolIncrease
	}

	fullyDiluted := totalExisting + newShares + optionPoolIncrease

	// Build ownership rows.
	ownership := make([]OwnershipRow, 0, len(existingShares)+2)
	for _, row := range existingShares {
		pctBefore := float64(row.SharesBefore) / float64(totalExisting) * 100.0
		pctAfter := float64(row.SharesBefore) / float64(fullyDiluted) * 100.0
		ownership = append(ownership, OwnershipRow{
			Name:          row.Name,
			SharesBefore:  row.SharesBefore,
			PercentBefore: pctBefore,
			SharesAfter:   row.SharesBefore,
			PercentAfter:  pctAfter,
			Dilution:      pctBefore - pctAfter,
		})
	}

	// New investor row.
	investorPctAfter := float64(newShares) / float64(fullyDiluted) * 100.0
	ownership = append(ownership, OwnershipRow{
		Name:          round.Name + " Investor",
		SharesBefore:  0,
		PercentBefore: 0,
		SharesAfter:   newShares,
		PercentAfter:  investorPctAfter,
		Dilution:      0,
	})

	// Option pool row (if expanded).
	if optionPoolIncrease > 0 {
		poolPctAfter := float64(optionPoolIncrease) / float64(fullyDiluted) * 100.0
		ownership = append(ownership, OwnershipRow{
			Name:          "Option Pool",
			SharesBefore:  0,
			PercentBefore: 0,
			SharesAfter:   optionPoolIncrease,
			PercentAfter:  poolPctAfter,
			Dilution:      0,
		})
	}

	return &ProFormaResult{
		PreMoneyValuation:  round.PreMoneyValuation,
		PostMoneyValuation: postMoney,
		PricePerShare:      pricePerShare,
		NewSharesIssued:    newShares,
		OptionPoolIncrease: optionPoolIncrease,
		Ownership:          ownership,
		FullyDilutedShares: fullyDiluted,
		OptionPoolShares:   optionPoolShares,
	}, nil
}

// ModelExit computes a simple pro-rata exit distribution.
// For waterfall-based distributions with liquidation preferences, use the waterfall package.
func ModelExit(shares []OwnershipRow, exitValuation int64) (*ExitScenario, error) {
	if exitValuation <= 0 {
		return nil, fmt.Errorf("exit_valuation must be positive")
	}
	if len(shares) == 0 {
		return nil, fmt.Errorf("shares must not be empty")
	}

	var totalShares int64
	for _, row := range shares {
		totalShares += row.SharesAfter
	}
	if totalShares <= 0 {
		return nil, fmt.Errorf("total shares must be positive")
	}

	exitRows := make([]ExitOwnershipRow, 0, len(shares))
	for _, row := range shares {
		if row.SharesAfter == 0 {
			continue
		}
		pct := float64(row.SharesAfter) / float64(totalShares)
		proceeds := pct * float64(exitValuation)

		var returnMultiple float64
		// Return multiple is only meaningful if the investor had a cost basis.
		// We use SharesBefore as a proxy: if shares increased, cost basis is new shares * implied price.
		// For simplicity, return multiple = proceeds / (shares * some price), but without price
		// info we just report proceeds/exit ratio.
		returnMultiple = 0 // caller must interpret based on context

		exitRows = append(exitRows, ExitOwnershipRow{
			Name:           row.Name,
			Proceeds:       proceeds,
			ReturnMultiple: returnMultiple,
			PercentOfExit:  pct * 100.0,
		})
	}

	return &ExitScenario{
		ExitValuation: exitValuation,
		Ownership:     exitRows,
	}, nil
}
