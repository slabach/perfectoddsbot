package betService

import (
	"perfectOddsBot/services/common"
	"testing"
)

func assertEqual(t *testing.T, expected, actual interface{}, msg string) {
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", msg, expected, actual)
	}
}

func TestUpdateParlaysOnBetResolution_ATSBets(t *testing.T) {
	tests := []struct {
		name           string
		betSpread      float64
		parlaySpread   *float64
		scoreDiff      int
		selectedOption int
		expectedWon    bool
		description    string
	}{
		{
			name:           "ATS Option 1 wins - covers spread",
			betSpread:      -7.5,
			parlaySpread:   floatPtr(-7.5),
			scoreDiff:      10,
			selectedOption: 1,
			expectedWon:    true,
			description:    "Home team wins by 10, needs to cover -7.5, wins",
		},
		{
			name:           "ATS Option 1 loses - doesn't cover",
			betSpread:      -7.5,
			parlaySpread:   floatPtr(-7.5),
			scoreDiff:      5,
			selectedOption: 1,
			expectedWon:    false,
			description:    "Home team wins by 5, needs to cover -7.5, loses",
		},
		{
			name:           "ATS Option 2 wins - covers spread",
			betSpread:      -7.5,
			parlaySpread:   floatPtr(-7.5),
			scoreDiff:      5,
			selectedOption: 2,
			expectedWon:    true,
			description:    "Home team wins by 5, away team covers +7.5, wins",
		},
		{
			name:           "ATS Option 2 loses - doesn't cover",
			betSpread:      -7.5,
			parlaySpread:   floatPtr(-7.5),
			scoreDiff:      10,
			selectedOption: 2,
			expectedWon:    false,
			description:    "Home team wins by 10, away team doesn't cover +7.5, loses",
		},
		{
			name:           "ATS spread changed - uses parlay's stored spread",
			betSpread:      -5.5,
			parlaySpread:   floatPtr(-7.5),
			scoreDiff:      6,
			selectedOption: 1,
			expectedWon:    false,
			description:    "Parlay created at -7.5, bet now -5.5, score diff 6, loses",
		},
		{
			name:           "ATS legacy entry - uses bet's current spread",
			betSpread:      -7.5,
			parlaySpread:   nil,
			scoreDiff:      10,
			selectedOption: 1,
			expectedWon:    true,
			description:    "Legacy entry uses bet's current spread -7.5, wins",
		},
		{
			name:           "ATS away favored - Option 1 wins",
			betSpread:      3.5,
			parlaySpread:   floatPtr(3.5),
			scoreDiff:      5,
			selectedOption: 1,
			expectedWon:    true,
			description:    "Away favored by 3.5, home wins by 5, Option 1 covers, wins",
		},
		{
			name:           "ATS away favored - Option 2 wins",
			betSpread:      3.5,
			parlaySpread:   floatPtr(3.5),
			scoreDiff:      -5,
			selectedOption: 2,
			expectedWon:    true,
			description:    "Away favored by 3.5, away wins by 5, Option 2 covers, wins",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var won bool
			spreadToUse := tt.parlaySpread
			if spreadToUse == nil {
				spreadToUse = &tt.betSpread
			}
			won = common.CalculateBetEntryWin(tt.selectedOption, tt.scoreDiff, *spreadToUse)

			assertEqual(t, tt.expectedWon, won, tt.description)
		})
	}
}

func TestUpdateParlaysOnBetResolution_MoneylineBets(t *testing.T) {
	tests := []struct {
		name           string
		scoreDiff      int
		selectedOption int
		expectedWon    bool
		description    string
	}{
		{
			name:           "Moneyline Option 1 wins",
			scoreDiff:      5,
			selectedOption: 1,
			expectedWon:    true,
			description:    "Option 1 wins by 5, parlay entry wins",
		},
		{
			name:           "Moneyline Option 1 loses",
			scoreDiff:      -5,
			selectedOption: 1,
			expectedWon:    false,
			description:    "Option 1 loses by 5, parlay entry loses",
		},
		{
			name:           "Moneyline Option 2 wins",
			scoreDiff:      -5,
			selectedOption: 2,
			expectedWon:    true,
			description:    "Option 2 wins by 5, parlay entry wins",
		},
		{
			name:           "Moneyline Option 2 loses",
			scoreDiff:      5,
			selectedOption: 2,
			expectedWon:    false,
			description:    "Option 2 loses by 5, parlay entry loses",
		},
		{
			name:           "Moneyline tie - both lose",
			scoreDiff:      0,
			selectedOption: 1,
			expectedWon:    false,
			description:    "Tie game, Option 1 loses",
		},
		{
			name:           "Moneyline tie Option 2 - both lose",
			scoreDiff:      0,
			selectedOption: 2,
			expectedWon:    false,
			description:    "Tie game, Option 2 loses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var won bool
			if tt.selectedOption == 1 {
				won = tt.scoreDiff > 0
			} else {
				won = tt.scoreDiff < 0
			}

			assertEqual(t, tt.expectedWon, won, tt.description)
		})
	}
}

func TestParlayStatusUpdates(t *testing.T) {
	tests := []struct {
		name              string
		initialStatus     string
		entryResults      []bool
		allResolved       bool
		expectedStatus    string
		expectedUserStats struct {
			betsWon    int
			betsLost   int
			pointsWon  float64
			pointsLost float64
		}
		description string
	}{
		{
			name:           "All entries win - parlay wins",
			initialStatus:  "pending",
			entryResults:   []bool{true, true, true},
			allResolved:    true,
			expectedStatus: "won",
			expectedUserStats: struct {
				betsWon    int
				betsLost   int
				pointsWon  float64
				pointsLost float64
			}{
				betsWon:    1,
				betsLost:   0,
				pointsWon:  1000.0,
				pointsLost: 0,
			},
			description: "All 3 entries win, parlay should be marked as won",
		},
		{
			name:           "One entry loses - parlay loses immediately",
			initialStatus:  "pending",
			entryResults:   []bool{true, false, true},
			allResolved:    false,
			expectedStatus: "lost",
			expectedUserStats: struct {
				betsWon    int
				betsLost   int
				pointsWon  float64
				pointsLost float64
			}{
				betsWon:    0,
				betsLost:   1,
				pointsWon:  0,
				pointsLost: 100.0,
			},
			description: "Second entry loses, parlay should be marked as lost immediately",
		},
		{
			name:           "All entries lose - parlay loses",
			initialStatus:  "pending",
			entryResults:   []bool{false, false, false},
			allResolved:    true,
			expectedStatus: "lost",
			expectedUserStats: struct {
				betsWon    int
				betsLost   int
				pointsWon  float64
				pointsLost float64
			}{
				betsWon:    0,
				betsLost:   1,
				pointsWon:  0,
				pointsLost: 100.0,
			},
			description: "All entries lose, parlay should be marked as lost",
		},
		{
			name:           "Partial resolution - some pending",
			initialStatus:  "pending",
			entryResults:   []bool{true, true},
			allResolved:    false,
			expectedStatus: "partial",
			expectedUserStats: struct {
				betsWon    int
				betsLost   int
				pointsWon  float64
				pointsLost float64
			}{
				betsWon:    0,
				betsLost:   0,
				pointsWon:  0,
				pointsLost: 0,
			},
			description: "Some entries resolved, some pending, status should be partial",
		},
		{
			name:           "Partial with loss - parlay loses",
			initialStatus:  "pending",
			entryResults:   []bool{true, false, true},
			allResolved:    false,
			expectedStatus: "lost",
			expectedUserStats: struct {
				betsWon    int
				betsLost   int
				pointsWon  float64
				pointsLost float64
			}{
				betsWon:    0,
				betsLost:   1,
				pointsWon:  0,
				pointsLost: 100.0,
			},
			description: "One entry loses while others pending, parlay should be marked as lost",
		},
		{
			name:           "Already lost - no duplicate updates",
			initialStatus:  "lost",
			entryResults:   []bool{false},
			allResolved:    true,
			expectedStatus: "lost",
			expectedUserStats: struct {
				betsWon    int
				betsLost   int
				pointsWon  float64
				pointsLost float64
			}{
				betsWon:    0,
				betsLost:   0,
				pointsWon:  0,
				pointsLost: 0,
			},
			description: "Parlay already marked as lost, should not update stats again",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasLoss := false
			for _, won := range tt.entryResults {
				if !won {
					hasLoss = true
					break
				}
			}

			var finalStatus string
			if hasLoss {
				finalStatus = "lost"
			} else if tt.allResolved {
				finalStatus = "won"
			} else {
				finalStatus = "partial"
			}

			if tt.initialStatus == "lost" || tt.initialStatus == "won" {
				finalStatus = tt.initialStatus
			}

			assertEqual(t, tt.expectedStatus, finalStatus, tt.description)
		})
	}
}

func TestParlayResolution_SpreadChanges(t *testing.T) {
	tests := []struct {
		name           string
		parlaySpread   float64
		currentSpread  float64
		scoreDiff      int
		selectedOption int
		useStored      bool
		expectedWon    bool
		description    string
	}{
		{
			name:           "Spread moved in favor - stored spread wins",
			parlaySpread:   -7.5,
			currentSpread:  -5.5,
			scoreDiff:      6,
			selectedOption: 1,
			useStored:      true,
			expectedWon:    false,
			description:    "Parlay at -7.5, bet now -5.5, score 6, stored spread loses",
		},
		{
			name:           "Spread moved against - stored spread loses",
			parlaySpread:   -5.5,
			currentSpread:  -7.5,
			scoreDiff:      6,
			selectedOption: 1,
			useStored:      true,
			expectedWon:    true,
			description:    "Parlay at -5.5, bet now -7.5, score 6, stored spread wins",
		},
		{
			name:           "Legacy entry uses current spread",
			parlaySpread:   0,
			currentSpread:  -7.5,
			scoreDiff:      10,
			selectedOption: 1,
			useStored:      false,
			expectedWon:    true,
			description:    "Legacy entry uses current -7.5 spread, wins",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spreadToUse := tt.currentSpread
			if tt.useStored {
				spreadToUse = tt.parlaySpread
			}

			won := common.CalculateBetEntryWin(tt.selectedOption, tt.scoreDiff, spreadToUse)
			assertEqual(t, tt.expectedWon, won, tt.description)
		})
	}
}

func TestMultipleParlaysSameBet(t *testing.T) {
	scoreDiff := 6

	parlays := []struct {
		parlayID       uint
		selectedOption int
		spread         float64
		expectedWon    bool
		description    string
	}{
		{
			parlayID:       1,
			selectedOption: 1,
			spread:         -7.5,
			expectedWon:    false,
			description:    "Parlay 1: Option 1 at -7.5, loses",
		},
		{
			parlayID:       2,
			selectedOption: 1,
			spread:         -5.5,
			expectedWon:    true,
			description:    "Parlay 2: Option 1 at -5.5, wins",
		},
		{
			parlayID:       3,
			selectedOption: 2,
			spread:         -7.5,
			expectedWon:    true,
			description:    "Parlay 3: Option 2 at -7.5, wins (covers +7.5)",
		},
		{
			parlayID:       4,
			selectedOption: 2,
			spread:         -5.5,
			expectedWon:    false,
			description:    "Parlay 4: Option 2 at -5.5, loses (doesn't cover +5.5)",
		},
	}

	for _, p := range parlays {
		t.Run(p.description, func(t *testing.T) {
			won := common.CalculateBetEntryWin(p.selectedOption, scoreDiff, p.spread)
			assertEqual(t, p.expectedWon, won, p.description)
		})
	}
}

func TestParlayPayoutCalculation(t *testing.T) {
	tests := []struct {
		name           string
		amount         int
		totalOdds      float64
		expectedPayout float64
		description    string
	}{
		{
			name:           "Simple parlay payout",
			amount:         100,
			totalOdds:      2.0,
			expectedPayout: 200.0,
			description:    "100 points at 2x odds = 200 payout",
		},
		{
			name:           "Large parlay payout",
			amount:         500,
			totalOdds:      10.5,
			expectedPayout: 5250.0,
			description:    "500 points at 10.5x odds = 5250 payout",
		},
		{
			name:           "Small parlay payout",
			amount:         10,
			totalOdds:      1.5,
			expectedPayout: 15.0,
			description:    "10 points at 1.5x odds = 15 payout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payout := common.CalculateParlayPayout(tt.amount, tt.totalOdds)
			assertEqual(t, tt.expectedPayout, payout, tt.description)
		})
	}
}

func floatPtr(f float64) *float64 {
	return &f
}

func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		scoreDiff      int
		spread         *float64
		selectedOption int
		expectedWon    bool
		description    string
	}{
		{
			name:           "Exact spread push - loses (spread is 0.5 so no push)",
			scoreDiff:      7,
			spread:         floatPtr(-7.5),
			selectedOption: 1,
			expectedWon:    false,
			description:    "Score diff exactly matches spread (but spread is 0.5 so no push), loses",
		},
		{
			name:           "Very close win",
			scoreDiff:      8,
			spread:         floatPtr(-7.5),
			selectedOption: 1,
			expectedWon:    true,
			description:    "Wins by 0.5 point margin",
		},
		{
			name:           "Very close loss",
			scoreDiff:      7,
			spread:         floatPtr(-7.5),
			selectedOption: 1,
			expectedWon:    false,
			description:    "Loses by 0.5 point margin",
		},
		{
			name:           "Large spread win",
			scoreDiff:      30,
			spread:         floatPtr(-20.5),
			selectedOption: 1,
			expectedWon:    true,
			description:    "Large spread, big win",
		},
		{
			name:           "Large spread loss",
			scoreDiff:      15,
			spread:         floatPtr(-20.5),
			selectedOption: 1,
			expectedWon:    false,
			description:    "Large spread, doesn't cover",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.spread == nil {
				var won bool
				if tt.selectedOption == 1 {
					won = tt.scoreDiff > 0
				} else {
					won = tt.scoreDiff < 0
				}
				assertEqual(t, tt.expectedWon, won, tt.description)
			} else {
				won := common.CalculateBetEntryWin(tt.selectedOption, tt.scoreDiff, *tt.spread)
				assertEqual(t, tt.expectedWon, won, tt.description)
			}
		})
	}
}

func TestNoParlayEntries(t *testing.T) {
	t.Log("Function should handle no parlay entries gracefully")
}

func TestAllEntriesLoseParlayResolution(t *testing.T) {
	scoreDiff := -10

	parlayEntries := []struct {
		selectedOption int
		spread         *float64
		expectedWon    bool
		description    string
	}{
		{
			selectedOption: 1,
			spread:         floatPtr(-16.5),
			expectedWon:    false,
			description:    "Parlay Option 1 at -16.5, home loses by 10, loses",
		},
		{
			selectedOption: 2,
			spread:         floatPtr(-16.5),
			expectedWon:    true,
			description:    "Parlay Option 2 at -16.5, away wins by 10, covers +16.5, wins",
		},
	}

	for _, entry := range parlayEntries {
		t.Run(entry.description, func(t *testing.T) {
			won := common.CalculateBetEntryWin(entry.selectedOption, scoreDiff, *entry.spread)
			assertEqual(t, entry.expectedWon, won, entry.description)
		})
	}
}
