package cards

// Shadows of the Syndicate decision math. Resolvers and tests share these
// functions so a boundary change cannot drift between them.

const (
	PointShaveAmount  = 1.5
	FixPushMaxMiss    = 3.0
	LoadedDiceMin     = 0.85
	LoadedDiceMax     = 1.25
	HoochMultiplier   = 1.25
	BagmanRate        = 0.15
	BrassKnucklesRate = 0.05
	RacketRate        = 0.20
	DonsFavorRate     = 0.25
	StreetTaxAmount   = 15.0
)

// ShiftSpreadInFavor moves a snapshot 1.5 points toward the bettor's side.
// Option 1 (home) gets +1.5. Option 2 gets -1.5. Both are easier covers
// under CalculateBetEntryWin.
func ShiftSpreadInFavor(spread float64, option int) float64 {
	if option == 2 {
		return spread - PointShaveAmount
	}
	return spread + PointShaveAmount
}

// ATSMiss is how many points a spread entry lost by. Zero or less is a cover
// (or a tie on the number, which this bot already treats as a loss, not a push).
// Option 1 miss is -spread - scoreDiff. Option 2 miss is scoreDiff + spread.
func ATSMiss(option int, scoreDiff, spread float64) float64 {
	if option == 2 {
		return scoreDiff + spread
	}
	return -spread - scoreDiff
}

// IsFixPush is a loss by more than 0 and at most 3 points.
func IsFixPush(miss float64) bool {
	return miss > 0 && miss <= FixPushMaxMiss
}

// ValidLoadedDice reports whether a stored roll can be applied. Zero and
// anything outside [0.85, 1.25] is ignored so a bad row cannot zero a payout.
func ValidLoadedDice(multiplier float64) bool {
	return multiplier >= LoadedDiceMin && multiplier <= LoadedDiceMax
}

// AmericanToDecimal matches one leg of CalculateParlayOddsMultiplier.
// Zero odds return 0 so the caller can fail closed.
func AmericanToDecimal(odds int) float64 {
	if odds == 0 {
		return 0
	}
	if odds > 0 {
		return (float64(odds) / 100.0) + 1.0
	}
	return (100.0 / float64(-odds)) + 1.0
}

// SpeakeasyPayout cashes amount * (totalOdds / legDecimal). ok is false when
// the decimal is missing or zero; the ticket then settles as a normal loss.
func SpeakeasyPayout(amount int, totalOdds, legDecimal float64) (float64, bool) {
	if amount <= 0 || totalOdds <= 0 || legDecimal <= 0 {
		return 0, false
	}
	return float64(amount) * (totalOdds / legDecimal), true
}

// WinSkim takes the Don's cut of net profit first, then the racket's cut of
// the net that is still in hand. Neither cut can drive the payout below zero.
func WinSkim(payout, stake float64, applyDon, applyRacket bool) (remaining, donCut, racketCut float64) {
	if payout < 0 {
		payout = 0
	}
	net := payout - stake
	if net < 0 {
		net = 0
	}
	remaining = payout
	if applyDon {
		donCut = net * DonsFavorRate
		if donCut > remaining {
			donCut = remaining
		}
		remaining -= donCut
	}
	remainingNet := net - donCut
	if remainingNet < 0 {
		remainingNet = 0
	}
	if applyRacket {
		racketCut = remainingNet * RacketRate
		if racketCut > remaining {
			racketCut = remaining
		}
		remaining -= racketCut
	}
	return remaining, donCut, racketCut
}

// DonLossBill is 25% of the stake that was not refunded, floored at the payer's balance.
func DonLossBill(unreimbursed, payerBalance float64) float64 {
	if unreimbursed <= 0 || payerBalance <= 0 {
		return 0
	}
	bill := unreimbursed * DonsFavorRate
	if bill > payerBalance {
		return payerBalance
	}
	return bill
}

// HoochCost raises a price by 25%. A free draw stays free.
func HoochCost(cost float64) float64 {
	if cost <= 0 {
		return 0
	}
	return cost * HoochMultiplier
}

// BagmanSplit sends 15% of the price actually paid to the holder and the rest to the pool.
func BagmanSplit(paid float64) (holder, pool float64) {
	if paid <= 0 {
		return 0, 0
	}
	holder = paid * BagmanRate
	return holder, paid - holder
}
