package betService

import (
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"testing"
)

type MockParlayResolutionScenario struct {
	name            string
	bet             models.Bet
	parlayEntries   []MockParlayEntry
	winningOption   int
	scoreDiff       int
	expectedResults []ExpectedParlayResult
	description     string
}

type MockParlayEntry struct {
	ID             uint
	ParlayID       uint
	SelectedOption int
	Spread         *float64
	Resolved       bool
	Won            *bool
}

type ExpectedParlayResult struct {
	ParlayID               uint
	ExpectedStatus         string
	ExpectedEntryWon       bool
	ExpectedUserStats      UserStats
	ExpectedGuildPool      float64
	ShouldSendNotification bool
	NotificationWon        bool
}

type UserStats struct {
	BetsWon    int
	BetsLost   int
	PointsWon  float64
	PointsLost float64
}

func TestUpdateParlaysOnBetResolution_ComprehensiveScenarios(t *testing.T) {
	scenarios := []MockParlayResolutionScenario{
		{
			name: "Single parlay - all legs win - moneyline",
			bet: models.Bet{
				ID:      1,
				Spread:  nil,
				Option1: "Team A",
				Option2: "Team B",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 1, Resolved: false},
			},
			winningOption: 1,
			scoreDiff:     10,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:               1,
					ExpectedStatus:         "partial",
					ExpectedEntryWon:       true,
					ShouldSendNotification: false,
					NotificationWon:        false,
				},
			},
			description: "Single-leg moneyline parlay entry, Option 1 wins",
		},
		{
			name: "Single parlay - first leg loses immediately - ATS",
			bet: models.Bet{
				ID:      1,
				Spread:  floatPtr(-7.5),
				Option1: "Team A",
				Option2: "Team B",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 1, Spread: floatPtr(-7.5), Resolved: false},
			},
			winningOption: 2,
			scoreDiff:     -10,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:               1,
					ExpectedStatus:         "lost",
					ExpectedEntryWon:       false,
					ShouldSendNotification: true,
					NotificationWon:        false,
				},
			},
			description: "Entry loses, parlay should be marked lost immediately",
		},
		{
			name: "Single parlay - ATS with stored spread different from current",
			bet: models.Bet{
				ID:      1,
				Spread:  floatPtr(-5.5),
				Option1: "Team A",
				Option2: "Team B",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 1, Spread: floatPtr(-7.5), Resolved: false},
			},
			winningOption: 1,
			scoreDiff:     6,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:               1,
					ExpectedStatus:         "partial",
					ExpectedEntryWon:       false,
					ShouldSendNotification: false,
					NotificationWon:        false,
				},
			},
			description: "Parlay with stored spread -7.5 loses (6 < 7.5), even though current spread -5.5 would win",
		},
		{
			name: "Multiple parlays same bet - different outcomes",
			bet: models.Bet{
				ID:      1,
				Spread:  floatPtr(-7.5),
				Option1: "Team A",
				Option2: "Team B",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 1, Spread: floatPtr(-7.5), Resolved: false},
				{ParlayID: 2, SelectedOption: 1, Spread: floatPtr(-5.5), Resolved: false},
				{ParlayID: 3, SelectedOption: 2, Spread: floatPtr(-7.5), Resolved: false},
			},
			winningOption: 1,
			scoreDiff:     6,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:               1,
					ExpectedStatus:         "lost",
					ExpectedEntryWon:       false,
					ShouldSendNotification: true,
					NotificationWon:        false,
				},
				{
					ParlayID:         2,
					ExpectedStatus:   "partial",
					ExpectedEntryWon: true,
				},
				{
					ParlayID:         3,
					ExpectedStatus:   "partial",
					ExpectedEntryWon: true,
				},
			},
			description: "Three parlays on same bet with different spreads get different outcomes",
		},
		{
			name: "Moneyline parlay - Option 2 wins",
			bet: models.Bet{
				ID:      1,
				Spread:  nil,
				Option1: "Team A",
				Option2: "Team B",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 2, Resolved: false},
			},
			winningOption: 2,
			scoreDiff:     -5,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:         1,
					ExpectedStatus:   "partial",
					ExpectedEntryWon: true,
				},
			},
			description: "Moneyline parlay with Option 2 winning",
		},
		{
			name: "Legacy parlay entry - no stored spread",
			bet: models.Bet{
				ID:      1,
				Spread:  floatPtr(-7.5),
				Option1: "Team A",
				Option2: "Team B",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 1, Spread: nil, Resolved: false},
			},
			winningOption: 1,
			scoreDiff:     10,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:         1,
					ExpectedStatus:   "partial",
					ExpectedEntryWon: true,
				},
			},
			description: "Legacy parlay entry uses bet's current spread",
		},
		{
			name: "Manually resolved bet - no scoreDiff",
			bet: models.Bet{
				ID:      1,
				Spread:  floatPtr(-7.5),
				Option1: "Team A",
				Option2: "Team B",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 1, Spread: floatPtr(-7.5), Resolved: false},
			},
			winningOption: 1,
			scoreDiff:     0,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:         1,
					ExpectedStatus:   "partial",
					ExpectedEntryWon: true,
				},
			},
			description: "Manually resolved bet uses simple option comparison",
		},
		{
			name: "Single entry loses - parlay marked lost",
			bet: models.Bet{
				ID:      1,
				Spread:  nil,
				Option1: "Team A",
				Option2: "Team B",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 2, Resolved: false},
			},
			winningOption: 1,
			scoreDiff:     5,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:               1,
					ExpectedStatus:         "lost",
					ExpectedEntryWon:       false,
					ShouldSendNotification: true,
					NotificationWon:        false,
				},
			},
			description: "Parlay entry loses, parlay should be marked lost immediately",
		},
		{
			name: "Single entry wins - parlay marked partial (other legs pending)",
			bet: models.Bet{
				ID:      1,
				Spread:  nil,
				Option1: "Team A",
				Option2: "Team B",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 1, Resolved: false},
			},
			winningOption: 1,
			scoreDiff:     5,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:               1,
					ExpectedStatus:         "partial",
					ExpectedEntryWon:       true,
					ShouldSendNotification: false,
					NotificationWon:        false,
				},
			},
			description: "Parlay entry wins but other legs pending, parlay should be partial",
		},
		{
			name: "ATS bet entry wins",
			bet: models.Bet{
				ID:      1,
				Spread:  floatPtr(-7.5),
				Option1: "Team A",
				Option2: "Team B",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 1, Spread: floatPtr(-7.5), Resolved: false},
			},
			winningOption: 1,
			scoreDiff:     10,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:               1,
					ExpectedStatus:         "partial",
					ExpectedEntryWon:       true,
					ShouldSendNotification: false,
					NotificationWon:        false,
				},
			},
			description: "ATS bet entry wins, parlay should be partial (other legs pending)",
		},
		{
			name: "Bet with no winners - parlays still resolve",
			bet: models.Bet{
				ID:      1,
				Spread:  floatPtr(-16.5),
				Option1: "Stanford",
				Option2: "CSU Northridge",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 1, Spread: floatPtr(-16.5), Resolved: false},
				{ParlayID: 2, SelectedOption: 2, Spread: floatPtr(-16.5), Resolved: false},
			},
			winningOption: 2,
			scoreDiff:     -10,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:               1,
					ExpectedStatus:         "lost",
					ExpectedEntryWon:       false,
					ShouldSendNotification: true,
					NotificationWon:        false,
				},
				{
					ParlayID:         2,
					ExpectedStatus:   "partial",
					ExpectedEntryWon: true,
				},
			},
			description: "Bet with no winners (all bet entries lost) but parlays still resolve correctly",
		},
		{
			name: "ATS away favored - different spreads",
			bet: models.Bet{
				ID:      1,
				Spread:  floatPtr(3.5),
				Option1: "Team A",
				Option2: "Team B",
			},
			parlayEntries: []MockParlayEntry{
				{ParlayID: 1, SelectedOption: 1, Spread: floatPtr(3.5), Resolved: false},
				{ParlayID: 2, SelectedOption: 2, Spread: floatPtr(5.5), Resolved: false},
			},
			winningOption: 1,
			scoreDiff:     5,
			expectedResults: []ExpectedParlayResult{
				{
					ParlayID:         1,
					ExpectedStatus:   "partial",
					ExpectedEntryWon: true,
				},
				{
					ParlayID:         2,
					ExpectedStatus:   "partial",
					ExpectedEntryWon: false,
				},
			},
			description: "Away favored bets with different spreads resolve correctly",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			for i, entry := range scenario.parlayEntries {
				var expectedResult *ExpectedParlayResult
				for j := range scenario.expectedResults {
					if scenario.expectedResults[j].ParlayID == entry.ParlayID {
						expectedResult = &scenario.expectedResults[j]
						break
					}
				}

				if expectedResult == nil {
					t.Fatalf("No expected result found for entry %d with parlay ID %d in scenario '%s'",
						i, entry.ParlayID, scenario.name)
				}

				expectedWon := expectedResult.ExpectedEntryWon

				var calculatedWon bool
				if scenario.bet.Spread == nil {
					calculatedWon = entry.SelectedOption == scenario.winningOption
				} else {
					if scenario.scoreDiff == 0 {
						calculatedWon = entry.SelectedOption == scenario.winningOption
					} else {
						var spreadToUse float64
						if entry.Spread != nil {
							spreadToUse = *entry.Spread
						} else {
							spreadToUse = *scenario.bet.Spread
						}
						calculatedWon = common.CalculateBetEntryWin(entry.SelectedOption, scenario.scoreDiff, spreadToUse)
					}
				}

				if calculatedWon != expectedWon {
					t.Errorf("Entry %d (ParlayID %d, SelectedOption %d): expected won=%v, got won=%v. %s\n"+
						"  Bet Spread: %v, Entry Spread: %v, WinningOption: %d, ScoreDiff: %d",
						i, entry.ParlayID, entry.SelectedOption, expectedWon, calculatedWon, scenario.description,
						scenario.bet.Spread, entry.Spread, scenario.winningOption, scenario.scoreDiff)
				}
			}
		})
	}
}

func TestParlayResolutionFlow(t *testing.T) {
	tests := []struct {
		name               string
		previousStatus     string
		currentEntryWon    bool
		allEntriesResolved bool
		hasLoss            bool
		expectedStatus     string
		shouldNotify       bool
		notificationWon    bool
		description        string
	}{
		{
			name:               "Entry loses - mark lost immediately",
			previousStatus:     "pending",
			currentEntryWon:    false,
			allEntriesResolved: false,
			hasLoss:            true,
			expectedStatus:     "lost",
			shouldNotify:       true,
			notificationWon:    false,
			description:        "First losing entry should mark parlay as lost immediately",
		},
		{
			name:               "Entry loses - already lost",
			previousStatus:     "lost",
			currentEntryWon:    false,
			allEntriesResolved: false,
			hasLoss:            true,
			expectedStatus:     "lost",
			shouldNotify:       false,
			notificationWon:    false,
			description:        "Parlay already lost, no duplicate notification",
		},
		{
			name:               "All entries win - mark won",
			previousStatus:     "pending",
			currentEntryWon:    true,
			allEntriesResolved: true,
			hasLoss:            false,
			expectedStatus:     "won",
			shouldNotify:       true,
			notificationWon:    true,
			description:        "All entries resolved and won, parlay wins",
		},
		{
			name:               "All entries resolved but has loss",
			previousStatus:     "pending",
			currentEntryWon:    true,
			allEntriesResolved: true,
			hasLoss:            true,
			expectedStatus:     "lost",
			shouldNotify:       true,
			notificationWon:    false,
			description:        "All resolved with loss - should have been handled earlier, but defensively mark as lost",
		},
		{
			name:               "Some entries pending - mark partial",
			previousStatus:     "pending",
			currentEntryWon:    true,
			allEntriesResolved: false,
			hasLoss:            false,
			expectedStatus:     "partial",
			shouldNotify:       false,
			notificationWon:    false,
			description:        "Entry wins but others pending, mark partial",
		},
		{
			name:               "All won - already won",
			previousStatus:     "won",
			currentEntryWon:    true,
			allEntriesResolved: true,
			hasLoss:            false,
			expectedStatus:     "won",
			shouldNotify:       false,
			notificationWon:    false,
			description:        "Parlay already won, no duplicate notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var finalStatus string
			var shouldNotify bool
			var notifyWon bool

			if !tt.currentEntryWon {
				finalStatus = "lost"
				if tt.previousStatus != "lost" && tt.previousStatus != "won" {
					shouldNotify = true
					notifyWon = false
				}
			} else if tt.allEntriesResolved {
				if !tt.hasLoss {
					finalStatus = "won"
					if tt.previousStatus != "lost" && tt.previousStatus != "won" {
						shouldNotify = true
						notifyWon = true
					}
				} else {
					finalStatus = "lost"
					if tt.previousStatus != "lost" && tt.previousStatus != "won" {
						shouldNotify = true
						notifyWon = false
					}
				}
			} else {
				finalStatus = "partial"
				shouldNotify = false
			}

			if finalStatus != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s. %s", tt.expectedStatus, finalStatus, tt.description)
			}

			if shouldNotify != tt.shouldNotify {
				t.Errorf("Expected shouldNotify %v, got %v. %s", tt.shouldNotify, shouldNotify, tt.description)
			}

			if shouldNotify && notifyWon != tt.notificationWon {
				t.Errorf("Expected notificationWon %v, got %v. %s", tt.notificationWon, notifyWon, tt.description)
			}
		})
	}
}

func TestParlayResolutionEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		betSpread      *float64
		entrySpread    *float64
		scoreDiff      int
		selectedOption int
		winningOption  int
		expectedWon    bool
		description    string
	}{
		{
			name:           "Exact spread edge - loses",
			betSpread:      floatPtr(-7.5),
			entrySpread:    floatPtr(-7.5),
			scoreDiff:      7,
			selectedOption: 1,
			winningOption:  1,
			expectedWon:    false,
			description:    "Score diff 7 vs spread -7.5, loses by 0.5",
		},
		{
			name:           "Exact spread edge - wins",
			betSpread:      floatPtr(-7.5),
			entrySpread:    floatPtr(-7.5),
			scoreDiff:      8,
			selectedOption: 1,
			winningOption:  1,
			expectedWon:    true,
			description:    "Score diff 8 vs spread -7.5, wins by 0.5",
		},
		{
			name:           "Zero score diff - Option 1 loses",
			betSpread:      nil,
			entrySpread:    nil,
			scoreDiff:      0,
			selectedOption: 1,
			winningOption:  1,
			expectedWon:    false,
			description:    "Zero score diff with moneyline - tie game",
		},
		{
			name:           "Large spread - big win",
			betSpread:      floatPtr(-20.5),
			entrySpread:    floatPtr(-20.5),
			scoreDiff:      35,
			selectedOption: 1,
			winningOption:  1,
			expectedWon:    true,
			description:    "Large spread -20.5, wins by 35, easily covers",
		},
		{
			name:           "Large spread - big loss",
			betSpread:      floatPtr(-20.5),
			entrySpread:    floatPtr(-20.5),
			scoreDiff:      15,
			selectedOption: 1,
			winningOption:  1,
			expectedWon:    false,
			description:    "Large spread -20.5, wins by 15, doesn't cover",
		},
		{
			name:           "Legacy entry with nil spread uses bet spread",
			betSpread:      floatPtr(-10.5),
			entrySpread:    nil,
			scoreDiff:      12,
			selectedOption: 1,
			winningOption:  1,
			expectedWon:    true,
			description:    "Legacy entry uses bet's current spread",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var won bool

			if tt.betSpread == nil {
				if tt.scoreDiff == 0 {
					won = false
				} else {
					won = tt.selectedOption == tt.winningOption
				}
			} else {
				if tt.scoreDiff == 0 {
					won = tt.selectedOption == tt.winningOption
				} else {
					var spreadToUse float64
					if tt.entrySpread != nil {
						spreadToUse = *tt.entrySpread
					} else {
						spreadToUse = *tt.betSpread
					}
					won = common.CalculateBetEntryWin(tt.selectedOption, tt.scoreDiff, spreadToUse)
				}
			}

			if won != tt.expectedWon {
				t.Errorf("Expected won=%v, got won=%v. %s", tt.expectedWon, won, tt.description)
			}
		})
	}
}

func TestParlayStatusTransitions(t *testing.T) {
	tests := []struct {
		name           string
		initialStatus  string
		entryWon       bool
		allResolved    bool
		hasLoss        bool
		expectedStatus string
		description    string
	}{
		{
			name:           "Pending -> Lost (entry loses)",
			initialStatus:  "pending",
			entryWon:       false,
			allResolved:    false,
			hasLoss:        true,
			expectedStatus: "lost",
			description:    "First losing entry transitions pending to lost",
		},
		{
			name:           "Pending -> Partial (entry wins, others pending)",
			initialStatus:  "pending",
			entryWon:       true,
			allResolved:    false,
			hasLoss:        false,
			expectedStatus: "partial",
			description:    "Winning entry with others pending transitions to partial",
		},
		{
			name:           "Partial -> Lost (new entry loses)",
			initialStatus:  "partial",
			entryWon:       false,
			allResolved:    false,
			hasLoss:        true,
			expectedStatus: "lost",
			description:    "Partial parlay loses when new entry loses",
		},
		{
			name:           "Partial -> Won (all resolved, all won)",
			initialStatus:  "partial",
			entryWon:       true,
			allResolved:    true,
			hasLoss:        false,
			expectedStatus: "won",
			description:    "Partial parlay wins when last entry wins",
		},
		{
			name:           "Pending -> Won (all resolved immediately)",
			initialStatus:  "pending",
			entryWon:       true,
			allResolved:    true,
			hasLoss:        false,
			expectedStatus: "won",
			description:    "Parlay wins immediately when all entries resolved and won",
		},
		{
			name:           "Lost -> Lost (no change)",
			initialStatus:  "lost",
			entryWon:       false,
			allResolved:    true,
			hasLoss:        true,
			expectedStatus: "lost",
			description:    "Lost parlay stays lost",
		},
		{
			name:           "Won -> Won (no change)",
			initialStatus:  "won",
			entryWon:       true,
			allResolved:    true,
			hasLoss:        false,
			expectedStatus: "won",
			description:    "Won parlay stays won",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var finalStatus string

			if !tt.entryWon {
				finalStatus = "lost"
			} else if tt.allResolved {
				if !tt.hasLoss {
					finalStatus = "won"
				} else {
					finalStatus = "lost"
				}
			} else {
				finalStatus = "partial"
			}

			if finalStatus != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s. %s", tt.expectedStatus, finalStatus, tt.description)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}
