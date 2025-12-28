package services

import (
	"fmt"
	"log"
	"perfectOddsBot/models"
	"perfectOddsBot/services/betService"
	"perfectOddsBot/services/common"
	"time"

	"github.com/bwmarrin/discordgo"
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
		betsWon    int
		betsLost   int
		pointsWon  float64
		pointsLost float64
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

// RunHistoricalStatsMigration populates betting stats for all historical resolved bets
func ReRunHistoricalStatsMigration(db *gorm.DB) error {
	// Check if migration has already run
	var existingMigration models.Migration
	result := db.Where("name = ?", "rerun_historical_betting_stats").First(&existingMigration)
	if result.Error == nil && existingMigration.ID != 0 {
		log.Println("Historical betting stats migration has already been executed. Skipping.")
		return nil
	}

	log.Println("Starting historical betting stats migration...")

	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		return fmt.Errorf("error fetching users: %v", err)
	}

	for _, user := range users {
		user.TotalBetsWon = 0
		user.TotalBetsLost = 0
		user.TotalPointsWon = 0
		user.TotalPointsLost = 0

		var userBets []models.BetEntry
		if err := db.Where("user_id = ?", user.ID).Find(&userBets).Error; err != nil {
			log.Printf("Error fetching bets for user %d: %v", user.ID, err)
			continue
		}

		for _, betEntry := range userBets {
			var bet models.Bet
			if err := db.First(&bet, "id = ?", betEntry.BetID).Error; err != nil {
				log.Printf("Error fetching bet %d: %v", betEntry.BetID, err)
				continue
			}
			if betEntry.AutoCloseWin {
				payout := common.CalculatePayout(betEntry.Amount, betEntry.Option, bet)
				user.TotalBetsWon++
				user.TotalPointsWon += payout
			} else {
				user.TotalBetsLost++
				user.TotalPointsLost += float64(betEntry.Amount)
			}
		}

		if err := db.Save(&user).Error; err != nil {
			log.Printf("Error updating stats for user %d: %v", user.ID, err)
			continue
		}
	}

	// Get all resolved bets (paid = true)
	var resolvedBets []models.Bet
	if err := db.Where("paid = ?", true).Find(&resolvedBets).Error; err != nil {
		return fmt.Errorf("error fetching resolved bets: %v", err)
	}

	// Mark migration as complete
	migration := models.Migration{
		Name:       "rerun_historical_betting_stats",
		ExecutedAt: time.Now(),
	}
	if err := db.Create(&migration).Error; err != nil {
		return fmt.Errorf("error marking migration as complete: %v", err)
	}

	return nil
}

// FixParlayResolutionMigration fixes parlay resolution issues:
// 1. Adds Spread column to parlay_entries table (handled by AutoMigrate)
// 2. Backfills existing parlay entries with bet.Spread where Spread IS NULL
// 3. Resolves all unresolved parlay entries for resolved bets
// s can be nil to skip sending Discord notifications
func FixParlayResolutionMigration(s *discordgo.Session, db *gorm.DB) error {
	// Check if migration has already run
	var existingMigration models.Migration
	result := db.Where("name = ?", "fix_parlay_resolution").First(&existingMigration)
	if result.Error == nil && existingMigration.ID != 0 {
		log.Println("Fix parlay resolution migration has already been executed. Skipping.")
		return nil
	}

	log.Println("Starting fix parlay resolution migration...")

	// Step 1: Backfill Spread column for existing parlay entries
	// Update parlay entries where Spread IS NULL and the bet has a spread
	log.Println("Backfilling Spread column for existing parlay entries...")
	backfillResult := db.Exec(`
		UPDATE parlay_entries pe
		INNER JOIN bets b ON pe.bet_id = b.id
		SET pe.spread = b.spread
		WHERE pe.spread IS NULL AND b.spread IS NOT NULL
	`)
	if backfillResult.Error != nil {
		log.Printf("Error backfilling Spread column: %v", backfillResult.Error)
	} else {
		log.Printf("Backfilled Spread for %d parlay entries", backfillResult.RowsAffected)
	}

	// Step 2: Resolve unresolved parlay entries for resolved bets
	// Get all resolved bets
	var resolvedBets []models.Bet
	if err := db.Where("paid = ?", true).Find(&resolvedBets).Error; err != nil {
		return fmt.Errorf("error fetching resolved bets: %v", err)
	}

	log.Printf("Processing %d resolved bets to fix unresolved parlay entries...", len(resolvedBets))

	resolvedCount := 0
	for _, bet := range resolvedBets {
		// Get all unresolved parlay entries for this bet
		var unresolvedEntries []models.ParlayEntry
		if err := db.Where("bet_id = ? AND resolved = ?", bet.ID, false).Find(&unresolvedEntries).Error; err != nil {
			log.Printf("Error fetching unresolved parlay entries for bet %d: %v", bet.ID, err)
			continue
		}

		if len(unresolvedEntries) == 0 {
			continue
		}

		// Determine winning option from bet entries with AutoCloseWin
		var betEntries []models.BetEntry
		if err := db.Where("bet_id = ?", bet.ID).Find(&betEntries).Error; err != nil {
			log.Printf("Error fetching bet entries for bet %d: %v", bet.ID, err)
			continue
		}

		winningOption := 0
		for _, entry := range betEntries {
			if entry.AutoCloseWin {
				winningOption = entry.Option
				break
			}
		}

		// If we couldn't determine winning option from entries, skip this bet
		// (it might be a manually resolved bet where we can't determine the correct option)
		if winningOption == 0 {
			log.Printf("Could not determine winning option for bet %d, skipping parlay resolution", bet.ID)
			continue
		}

		// Resolve each parlay entry
		for _, entry := range unresolvedEntries {
			// Determine if this parlay entry won using simple option comparison
			// (since we don't have scoreDiff, this is the best we can do)
			won := entry.SelectedOption == winningOption

			// Mark entry as resolved
			entry.Resolved = true
			entry.Won = &won
			if err := db.Save(&entry).Error; err != nil {
				log.Printf("Error updating parlay entry %d: %v", entry.ID, err)
				continue
			}

			// Get the parlay and update its status
			var parlay models.Parlay
			if err := db.Preload("ParlayEntries").Preload("ParlayEntries.Bet").First(&parlay, entry.ParlayID).Error; err != nil {
				log.Printf("Error fetching parlay %d: %v", entry.ParlayID, err)
				continue
			}

			previousStatus := parlay.Status

			// Check if all entries are resolved
			allResolved := true
			hasLoss := false
			for _, pe := range parlay.ParlayEntries {
				if !pe.Resolved {
					allResolved = false
					break
				}
				if pe.Won != nil && !*pe.Won {
					hasLoss = true
				}
			}

			// Update parlay status
			if !won {
				// This entry lost, mark parlay as lost immediately
				parlay.Status = "lost"
				if err := db.Save(&parlay).Error; err != nil {
					log.Printf("Error updating parlay %d status: %v", parlay.ID, err)
					continue
				}

				// Update user stats if parlay just became fully resolved
				if previousStatus != "lost" && previousStatus != "won" {
					var user models.User
					if err := db.First(&user, parlay.UserID).Error; err == nil {
						user.TotalBetsLost++
						user.TotalPointsLost += float64(parlay.Amount)
						db.Save(&user)

						// Add to guild pool
						var guild models.Guild
						if err := db.Where("guild_id = ?", parlay.GuildID).First(&guild).Error; err == nil {
							db.Model(&models.Guild{}).Where("id = ?", guild.ID).UpdateColumn("pool", gorm.Expr("pool + ?", float64(parlay.Amount)))
						}
					}
					// Send notification if Discord session is available
					if s != nil {
						betService.SendParlayResolutionNotification(s, db, parlay, false)
					}
				}
			} else if allResolved {
				// All entries resolved, check if parlay won
				if hasLoss {
					parlay.Status = "lost"
					if err := db.Save(&parlay).Error; err != nil {
						log.Printf("Error updating parlay %d status: %v", parlay.ID, err)
						continue
					}

					// Update user stats if parlay just became fully resolved
					if previousStatus != "lost" && previousStatus != "won" {
						var user models.User
						if err := db.First(&user, parlay.UserID).Error; err == nil {
							user.TotalBetsLost++
							user.TotalPointsLost += float64(parlay.Amount)
							db.Save(&user)

							// Add to guild pool
							var guild models.Guild
							if err := db.Where("guild_id = ?", parlay.GuildID).First(&guild).Error; err == nil {
								db.Model(&models.Guild{}).Where("id = ?", guild.ID).UpdateColumn("pool", gorm.Expr("pool + ?", float64(parlay.Amount)))
							}
						}
					}
				} else {
					// All bets won! Calculate and pay out parlay
					parlay.Status = "won"
					if err := db.Save(&parlay).Error; err != nil {
						log.Printf("Error updating parlay %d status: %v", parlay.ID, err)
						continue
					}

					// Update user stats if parlay just became fully resolved
					if previousStatus != "lost" && previousStatus != "won" {
						var user models.User
						if err := db.First(&user, parlay.UserID).Error; err == nil {
							payout := common.CalculateParlayPayout(parlay.Amount, parlay.TotalOdds)
							user.Points += payout
							user.TotalBetsWon++
							user.TotalPointsWon += payout
							db.Save(&user)
						}
						// Send notification if Discord session is available
						if s != nil {
							betService.SendParlayResolutionNotification(s, db, parlay, true)
						}
					}
				}
			} else {
				// Some bets still pending
				parlay.Status = "partial"
				db.Save(&parlay)
			}

			resolvedCount++
		}
	}

	log.Printf("Resolved %d parlay entries in migration", resolvedCount)

	// Mark migration as complete
	migration := models.Migration{
		Name:       "fix_parlay_resolution",
		ExecutedAt: time.Now(),
	}
	if err := db.Create(&migration).Error; err != nil {
		return fmt.Errorf("error marking migration as complete: %v", err)
	}

	log.Println("Fix parlay resolution migration completed.")
	return nil
}
