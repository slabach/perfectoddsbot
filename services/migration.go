package services

import (
	"fmt"
	"log"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"time"

	"gorm.io/gorm"
)

// RunHistoricalStatsMigration populates betting stats for all historical resolved bets
func RunHistoricalStatsMigration(db *gorm.DB) error {
	// Check if migration has already run
	var existingMigration models.Migration
	result := db.Where("name = ?", "historical_betting_stats").First(&existingMigration)
	if result.Error == nil && existingMigration.ID != 0 {
		log.Println("Historical betting stats migration has already been executed. Skipping.")
		return nil
	}

	log.Println("Starting historical betting stats migration...")

	// Get all resolved bets (paid = true)
	var resolvedBets []models.Bet
	if err := db.Where("paid = ?", true).Find(&resolvedBets).Error; err != nil {
		return fmt.Errorf("error fetching resolved bets: %v", err)
	}

	// Map to aggregate stats per user: userID -> stats
	type userStats struct {
		betsWon      int
		betsLost     int
		pointsWon    float64
		pointsLost   float64
	}
	statsMap := make(map[uint]*userStats)

	// Process each resolved bet
	for _, bet := range resolvedBets {
		var entries []models.BetEntry
		if err := db.Where("bet_id = ?", bet.ID).Find(&entries).Error; err != nil {
			log.Printf("Error fetching entries for bet %d: %v", bet.ID, err)
			continue
		}

		// Check if this bet has AutoCloseWin flags set (auto-resolved bets)
		hasAutoCloseFlags := false
		for _, entry := range entries {
			if entry.AutoCloseWin {
				hasAutoCloseFlags = true
				break
			}
		}

		// Only process bets where we can determine winners (auto-resolved bets with AutoCloseWin flags)
		// For manually resolved bets without AutoCloseWin, we can't reliably determine winners from historical data
		if !hasAutoCloseFlags {
			log.Printf("Skipping bet %d: cannot determine winners (no AutoCloseWin flags)", bet.ID)
			continue
		}

		// Process entries for this bet
		for _, entry := range entries {
			// Initialize stats for this user if not exists
			if statsMap[entry.UserID] == nil {
				statsMap[entry.UserID] = &userStats{}
			}

			if entry.AutoCloseWin {
				// Winning entry
				payout := common.CalculatePayout(entry.Amount, entry.Option, bet)
				statsMap[entry.UserID].betsWon++
				statsMap[entry.UserID].pointsWon += payout
			} else {
				// Losing entry
				statsMap[entry.UserID].betsLost++
				statsMap[entry.UserID].pointsLost += float64(entry.Amount)
			}
		}
	}

	// Update all users with their aggregated stats
	for userID, stats := range statsMap {
		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			log.Printf("Error fetching user %d: %v", userID, err)
			continue
		}

		user.TotalBetsWon += stats.betsWon
		user.TotalBetsLost += stats.betsLost
		user.TotalPointsWon += stats.pointsWon
		user.TotalPointsLost += stats.pointsLost

		if err := db.Save(&user).Error; err != nil {
			log.Printf("Error updating stats for user %d: %v", userID, err)
			continue
		}
	}

	// Mark migration as complete
	migration := models.Migration{
		Name:       "historical_betting_stats",
		ExecutedAt: time.Now(),
	}
	if err := db.Create(&migration).Error; err != nil {
		return fmt.Errorf("error marking migration as complete: %v", err)
	}

	log.Printf("Historical betting stats migration completed. Updated %d users.", len(statsMap))
	return nil
}

