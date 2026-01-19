package scheduler_jobs

import (
	"perfectOddsBot/services/common"
	"testing"
)

func TestCalculateBetEntryWin(t *testing.T) {
	tests := []struct {
		name      string
		option    int
		scoreDiff int
		spread    float64
		expected  bool
		scenario  string
	}{
		// ===== HOME TEAM FAVORED (spread < 0) =====
		{
			name:      "Home favored Option1 wins - covers spread",
			option:    1,
			scoreDiff: 10,
			spread:    -3.5,
			expected:  true,
			scenario:  "Home team wins by 10, needs to cover -3.5, wins",
		},
		{
			name:      "Home favored Option1 wins - exact spread",
			option:    1,
			scoreDiff: 4,
			spread:    -3.5,
			expected:  true,
			scenario:  "Home team wins by 4, needs to cover -3.5, wins",
		},
		{
			name:      "Home favored Option1 loses - doesn't cover",
			option:    1,
			scoreDiff: 2,
			spread:    -3.5,
			expected:  false,
			scenario:  "Home team wins by 2, needs to cover -3.5, loses",
		},
		{
			name:      "Home favored Option1 loses - home loses game",
			option:    1,
			scoreDiff: -5,
			spread:    -3.5,
			expected:  false,
			scenario:  "Home team loses by 5, needs to cover -3.5, loses",
		},
		{
			name:      "Home favored Option2 wins - away covers",
			option:    2,
			scoreDiff: -2,
			spread:    -3.5,
			expected:  true,
			scenario:  "Away team wins by 2, home favored by 3.5, away covers, wins",
		},
		{
			name:      "Home favored Option2 loses - away doesn't cover",
			option:    2,
			scoreDiff: 5,
			spread:    -3.5,
			expected:  false,
			scenario:  "Home team wins by 5, home favored by 3.5, away doesn't cover, loses",
		},
		{
			name:      "Home favored Option2 loses - exact spread",
			option:    2,
			scoreDiff: 4,
			spread:    -3.5,
			expected:  false,
			scenario:  "Home team wins by 4, home favored by 3.5, away doesn't cover, loses",
		},

		// ===== AWAY TEAM FAVORED (spread > 0) =====
		{
			name:      "Away favored Option1 wins - home covers",
			option:    1,
			scoreDiff: 5,
			spread:    3.5,
			expected:  true,
			scenario:  "Home team wins by 5, away favored by 3.5, home covers, wins",
		},
		{
			name:      "Away favored Option1 wins - home covers despite losing",
			option:    1,
			scoreDiff: -2,
			spread:    3.5,
			expected:  true,
			scenario:  "Away team wins by 2, away favored by 3.5, home gets +3.5 so covers, wins",
		},
		{
			name:      "Away favored Option1 loses - exact spread",
			option:    1,
			scoreDiff: -4,
			spread:    3.5,
			expected:  false,
			scenario:  "Away team wins by 4, away favored by 3.5, home doesn't cover, loses",
		},
		{
			name:      "Away favored Option2 wins - covers spread (BUG CASE)",
			option:    2,
			scoreDiff: -9,
			spread:    3.5,
			expected:  true,
			scenario:  "Away team wins by 9, needs to cover 3.5, wins (original bug case)",
		},
		{
			name:      "Away favored Option2 wins - exact spread",
			option:    2,
			scoreDiff: -4,
			spread:    3.5,
			expected:  true,
			scenario:  "Away team wins by 4, needs to cover 3.5, wins",
		},
		{
			name:      "Away favored Option2 loses - doesn't cover",
			option:    2,
			scoreDiff: -2,
			spread:    3.5,
			expected:  false,
			scenario:  "Away team wins by 2, needs to cover 3.5, loses",
		},
		{
			name:      "Away favored Option2 loses - home wins game",
			option:    2,
			scoreDiff: 5,
			spread:    3.5,
			expected:  false,
			scenario:  "Home team wins by 5, away favored by 3.5, away loses",
		},

		// ===== NO SPREAD (spread = 0) =====
		{
			name:      "No spread Option1 wins - home wins",
			option:    1,
			scoreDiff: 5,
			spread:    0.0,
			expected:  true,
			scenario:  "Home team wins by 5, no spread, wins",
		},
		{
			name:      "No spread Option1 loses - home loses",
			option:    1,
			scoreDiff: -5,
			spread:    0.0,
			expected:  false,
			scenario:  "Home team loses by 5, no spread, loses",
		},
		{
			name:      "No spread Option2 wins - away wins",
			option:    2,
			scoreDiff: -5,
			spread:    0.0,
			expected:  true,
			scenario:  "Away team wins by 5, no spread, wins",
		},
		{
			name:      "No spread Option2 loses - away loses",
			option:    2,
			scoreDiff: 5,
			spread:    0.0,
			expected:  false,
			scenario:  "Away team loses by 5, no spread, loses",
		},
		{
			name:      "No spread tie - both lose",
			option:    1,
			scoreDiff: 0,
			spread:    0.0,
			expected:  false,
			scenario:  "Tie game, no spread, Option1 loses",
		},
		{
			name:      "No spread tie Option2 - both lose",
			option:    2,
			scoreDiff: 0,
			spread:    0.0,
			expected:  false,
			scenario:  "Tie game, no spread, Option2 loses",
		},

		// ===== LARGE SPREADS =====
		{
			name:      "Large home spread Option1 wins",
			option:    1,
			scoreDiff: 25,
			spread:    -20.5,
			expected:  true,
			scenario:  "Home team wins by 25, needs to cover -20.5, wins",
		},
		{
			name:      "Large home spread Option1 loses",
			option:    1,
			scoreDiff: 15,
			spread:    -20.5,
			expected:  false,
			scenario:  "Home team wins by 15, needs to cover -20.5, loses",
		},
		{
			name:      "Large away spread Option2 wins",
			option:    2,
			scoreDiff: -25,
			spread:    20.5,
			expected:  true,
			scenario:  "Away team wins by 25, needs to cover 20.5, wins",
		},
		{
			name:      "Large away spread Option2 loses",
			option:    2,
			scoreDiff: -15,
			spread:    20.5,
			expected:  false,
			scenario:  "Away team wins by 15, needs to cover 20.5, loses",
		},

		// ===== EDGE CASES - CLOSE GAMES =====
		{
			name:      "Close game Option1 wins by 0.5",
			option:    1,
			scoreDiff: 1,
			spread:    -0.5,
			expected:  true,
			scenario:  "Home team wins by 1, needs to cover -0.5, wins",
		},
		{
			name:      "Close game Option2 wins by 0.5",
			option:    2,
			scoreDiff: -1,
			spread:    0.5,
			expected:  true,
			scenario:  "Away team wins by 1, needs to cover 0.5, wins",
		},
		{
			name:      "Close game Option1 loses by 0.5",
			option:    1,
			scoreDiff: 0,
			spread:    -0.5,
			expected:  false,
			scenario:  "Tie game, home needs to cover -0.5, loses",
		},
		{
			name:      "Close game Option2 loses by 0.5",
			option:    2,
			scoreDiff: 0,
			spread:    0.5,
			expected:  false,
			scenario:  "Tie game, away needs to cover 0.5, loses",
		},

		// ===== NEGATIVE SPREAD EDGE CASES =====
		{
			name:      "Very negative spread Option1",
			option:    1,
			scoreDiff: 30,
			spread:    -25.5,
			expected:  true,
			scenario:  "Home team wins by 30, needs to cover -25.5, wins",
		},
		{
			name:      "Very positive spread Option2",
			option:    2,
			scoreDiff: -30,
			spread:    25.5,
			expected:  true,
			scenario:  "Away team wins by 30, needs to cover 25.5, wins",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := common.CalculateBetEntryWin(tt.option, tt.scoreDiff, tt.spread)
			if result != tt.expected {
				t.Errorf("CalculateBetEntryWin(option=%d, scoreDiff=%d, spread=%.1f) = %v, want %v\nScenario: %s",
					tt.option, tt.scoreDiff, tt.spread, result, tt.expected, tt.scenario)
			}
		})
	}
}
