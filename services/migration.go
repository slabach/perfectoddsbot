package services

import (
	"fmt"
	"log"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"reflect"
	"runtime"
	"strings"
	"time"

	"gorm.io/gorm"
)

// #region Historical Migrations
/*
func RunHistoricalStatsMigration(db *gorm.DB) error {
	var existingMigration models.Migration
	result := db.Where("name = ?", "historical_betting_stats").First(&existingMigration)
	if result.Error == nil && existingMigration.ID != 0 {
		log.Println("Historical betting stats migration has already been executed. Skipping.")
		return nil
	}

	log.Println("Starting historical betting stats migration...")

	var resolvedBets []models.Bet
	if err := db.Where("paid = ?", true).Find(&resolvedBets).Error; err != nil {
		return fmt.Errorf("error fetching resolved bets: %v", err)
	}

	type userStats struct {
		betsWon    int
		betsLost   int
		pointsWon  float64
		pointsLost float64
	}
	statsMap := make(map[uint]*userStats)

	for _, bet := range resolvedBets {
		var entries []models.BetEntry
		if err := db.Where("bet_id = ?", bet.ID).Find(&entries).Error; err != nil {
			log.Printf("Error fetching entries for bet %d: %v", bet.ID, err)
			continue
		}

		hasAutoCloseFlags := false
		for _, entry := range entries {
			if entry.AutoCloseWin {
				hasAutoCloseFlags = true
				break
			}
		}

		if !hasAutoCloseFlags {
			log.Printf("Skipping bet %d: cannot determine winners (no AutoCloseWin flags)", bet.ID)
			continue
		}

		for _, entry := range entries {
			if statsMap[entry.UserID] == nil {
				statsMap[entry.UserID] = &userStats{}
			}

			if entry.AutoCloseWin {
				payout := common.CalculatePayout(entry.Amount, entry.Option, bet)
				statsMap[entry.UserID].betsWon++
				statsMap[entry.UserID].pointsWon += payout
			} else {
				statsMap[entry.UserID].betsLost++
				statsMap[entry.UserID].pointsLost += float64(entry.Amount)
			}
		}
	}

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

func ReRunHistoricalStatsMigration(db *gorm.DB) error {
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

	var resolvedBets []models.Bet
	if err := db.Where("paid = ?", true).Find(&resolvedBets).Error; err != nil {
		return fmt.Errorf("error fetching resolved bets: %v", err)
	}

	migration := models.Migration{
		Name:       "rerun_historical_betting_stats",
		ExecutedAt: time.Now(),
	}
	if err := db.Create(&migration).Error; err != nil {
		return fmt.Errorf("error marking migration as complete: %v", err)
	}

	return nil
}

func FixParlayResolutionMigration(s *discordgo.Session, db *gorm.DB) error {
	var existingMigration models.Migration
	result := db.Where("name = ?", "fix_parlay_resolution").First(&existingMigration)
	if result.Error == nil && existingMigration.ID != 0 {
		log.Println("Fix parlay resolution migration has already been executed. Skipping.")
		return nil
	}

	log.Println("Starting fix parlay resolution migration...")

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

	var resolvedBets []models.Bet
	if err := db.Where("paid = ?", true).Find(&resolvedBets).Error; err != nil {
		return fmt.Errorf("error fetching resolved bets: %v", err)
	}

	log.Printf("Processing %d resolved bets to fix unresolved parlay entries...", len(resolvedBets))

	resolvedCount := 0
	for _, bet := range resolvedBets {
		var unresolvedEntries []models.ParlayEntry
		if err := db.Where("bet_id = ? AND resolved = ?", bet.ID, false).Find(&unresolvedEntries).Error; err != nil {
			log.Printf("Error fetching unresolved parlay entries for bet %d: %v", bet.ID, err)
			continue
		}

		if len(unresolvedEntries) == 0 {
			continue
		}

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

		if winningOption == 0 {
			log.Printf("Could not determine winning option for bet %d, skipping parlay resolution", bet.ID)
			continue
		}

		for _, entry := range unresolvedEntries {
			won := entry.SelectedOption == winningOption

			entry.Resolved = true
			entry.Won = &won
			if err := db.Save(&entry).Error; err != nil {
				log.Printf("Error updating parlay entry %d: %v", entry.ID, err)
				continue
			}

			var parlay models.Parlay
			if err := db.Preload("ParlayEntries").Preload("ParlayEntries.Bet").First(&parlay, entry.ParlayID).Error; err != nil {
				log.Printf("Error fetching parlay %d: %v", entry.ParlayID, err)
				continue
			}

			previousStatus := parlay.Status

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

			if !won {
				parlay.Status = "lost"
				if err := db.Save(&parlay).Error; err != nil {
					log.Printf("Error updating parlay %d status: %v", parlay.ID, err)
					continue
				}

				if previousStatus != "lost" && previousStatus != "won" {
					var user models.User
					if err := db.First(&user, parlay.UserID).Error; err == nil {
						user.TotalBetsLost++
						user.TotalPointsLost += float64(parlay.Amount)
						db.Save(&user)

						var guild *models.Guild
						var guildErr error
						if s != nil {
							guild, guildErr = guildService.GetGuildInfo(s, db, parlay.GuildID, "")
						} else {
							var g models.Guild
							guildErr = db.Where("guild_id = ?", parlay.GuildID).First(&g).Error
							if guildErr == nil {
								guild = &g
							}
						}
						if guildErr == nil && guild != nil {
							db.Model(&models.Guild{}).Where("id = ?", guild.ID).UpdateColumn("pool", gorm.Expr("pool + ?", float64(parlay.Amount)))
						}
					}
					if s != nil {
						betService.SendParlayResolutionNotification(s, db, parlay, false)
					}
				}
			} else if allResolved {
				if hasLoss {
					parlay.Status = "lost"
					if err := db.Save(&parlay).Error; err != nil {
						log.Printf("Error updating parlay %d status: %v", parlay.ID, err)
						continue
					}

					if previousStatus != "lost" && previousStatus != "won" {
						var user models.User
						if err := db.First(&user, parlay.UserID).Error; err == nil {
							user.TotalBetsLost++
							user.TotalPointsLost += float64(parlay.Amount)
							db.Save(&user)

							var guild *models.Guild
							var guildErr error
							if s != nil {
								guild, guildErr = guildService.GetGuildInfo(s, db, parlay.GuildID, "")
							} else {
								var g models.Guild
								guildErr = db.Where("guild_id = ?", parlay.GuildID).First(&g).Error
								if guildErr == nil {
									guild = &g
								}
							}
							if guildErr == nil && guild != nil {
								db.Model(&models.Guild{}).Where("id = ?", guild.ID).UpdateColumn("pool", gorm.Expr("pool + ?", float64(parlay.Amount)))
							}
						}
					}
				} else {
					parlay.Status = "won"
					if err := db.Save(&parlay).Error; err != nil {
						log.Printf("Error updating parlay %d status: %v", parlay.ID, err)
						continue
					}

					if previousStatus != "lost" && previousStatus != "won" {
						var user models.User
						if err := db.First(&user, parlay.UserID).Error; err == nil {
							payout := common.CalculateParlayPayout(parlay.Amount, parlay.TotalOdds)
							user.Points += payout
							user.TotalBetsWon++
							user.TotalPointsWon += payout
							db.Save(&user)
						}
						if s != nil {
							betService.SendParlayResolutionNotification(s, db, parlay, true)
						}
					}
				}
			} else {
				parlay.Status = "partial"
				db.Save(&parlay)
			}

			resolvedCount++
		}
	}

	log.Printf("Resolved %d parlay entries in migration", resolvedCount)

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
*/
// #endregion

func RunUserInventoryTimesAppliedBackfill(db *gorm.DB) error {
	const migrationName = "user_inventory_times_applied_backfill"
	var existing models.Migration
	if err := db.Where("name = ?", migrationName).First(&existing).Error; err == nil && existing.ID != 0 {
		log.Println("User inventory times_applied backfill already executed. Skipping.")
		return nil
	}

	log.Println("Backfilling user_inventories.times_applied: setting NULL to 0...")
	res := db.Exec("UPDATE user_inventories SET times_applied = 0 WHERE times_applied IS NULL")
	if res.Error != nil {
		return fmt.Errorf("user_inventory times_applied backfill: %w", res.Error)
	}
	log.Printf("User inventory times_applied backfill completed. Rows updated: %d", res.RowsAffected)

	if err := db.Create(&models.Migration{Name: migrationName, ExecutedAt: time.Now()}).Error; err != nil {
		return fmt.Errorf("error recording times_applied backfill migration: %w", err)
	}
	return nil
}

func RunUserInventoryCardCodeBackfill(db *gorm.DB) error {
	const migrationName = "user_inventory_card_code_backfill"
	var existing models.Migration
	if err := db.Where("name = ?", migrationName).First(&existing).Error; err == nil && existing.ID != 0 {
		log.Println("User inventory card code backfill already executed. Skipping.")
		return nil
	}
	log.Println("Backfilling user_inventories.card_code: setting NULL to card code...")
	res := db.Exec(`
		UPDATE user_inventories 
		SET card_code = (SELECT code FROM cards WHERE id = user_inventories.card_id)
		WHERE EXISTS (
			SELECT 1 FROM cards 
			WHERE cards.id = user_inventories.card_id 
			AND cards.code IS NOT NULL
		)
	`)
	if res.Error != nil {
		return fmt.Errorf("user_inventory card code backfill: %w", res.Error)
	}
	log.Printf("User inventory card code backfill completed. Rows updated: %d", res.RowsAffected)

	if err := db.Create(&models.Migration{Name: migrationName, ExecutedAt: time.Now()}).Error; err != nil {
		return fmt.Errorf("error recording user_inventory_card_code_backfill migration: %w", err)
	}
	return nil
}

func RunVampireDevilExpiresAtBackfill(db *gorm.DB) error {
	const migrationName = "vampire_devil_expires_at_backfill"
	var existing models.Migration
	if err := db.Where("name = ?", migrationName).First(&existing).Error; err == nil && existing.ID != 0 {
		log.Println("Vampire/Devil expires_at backfill already executed. Skipping.")
		return nil
	}

	log.Println("Backfilling expires_at for legacy Vampire and The Devil inventory rows...")

	res := db.Exec(
		"UPDATE user_inventories SET expires_at = DATE_ADD(created_at, INTERVAL 24 HOUR) WHERE card_id = ? AND expires_at IS NULL AND deleted_at IS NULL",
		cards.VampireCardID,
	)
	if res.Error != nil {
		return fmt.Errorf("vampire expires_at backfill: %w", res.Error)
	}
	vampireUpdated := res.RowsAffected

	res = db.Exec(
		"UPDATE user_inventories SET expires_at = DATE_ADD(created_at, INTERVAL 7 DAY) WHERE card_id = ? AND expires_at IS NULL AND deleted_at IS NULL",
		cards.TheDevilCardID,
	)
	if res.Error != nil {
		return fmt.Errorf("devil expires_at backfill: %w", res.Error)
	}
	devilUpdated := res.RowsAffected

	log.Printf("Vampire/Devil expires_at backfill completed. Vampire: %d rows, Devil: %d rows.", vampireUpdated, devilUpdated)

	if err := db.Create(&models.Migration{Name: migrationName, ExecutedAt: time.Now()}).Error; err != nil {
		return fmt.Errorf("error recording vampire_devil_expires_at_backfill migration: %w", err)
	}
	return nil
}

func RunHomeFieldAdvantageExpiresAtBackfill(db *gorm.DB) error {
	const migrationName = "home_field_advantage_expires_at_backfill"
	var existing models.Migration
	if err := db.Where("name = ?", migrationName).First(&existing).Error; err == nil && existing.ID != 0 {
		log.Println("Home Field Advantage expires_at backfill already executed. Skipping.")
		return nil
	}

	log.Println("Backfilling expires_at for legacy Home Field Advantage inventory rows...")

	res := db.Exec(
		"UPDATE user_inventories SET expires_at = DATE_ADD(created_at, INTERVAL 24 HOUR) WHERE card_id = ? AND expires_at IS NULL AND deleted_at IS NULL",
		cards.HomeFieldAdvantageCardID,
	)
	if res.Error != nil {
		return fmt.Errorf("home field advantage expires_at backfill: %w", res.Error)
	}
	rowsUpdated := res.RowsAffected

	log.Printf("Home Field Advantage expires_at backfill completed. Rows updated: %d", rowsUpdated)

	if err := db.Create(&models.Migration{Name: migrationName, ExecutedAt: time.Now()}).Error; err != nil {
		return fmt.Errorf("error recording home_field_advantage_expires_at_backfill migration: %w", err)
	}
	return nil
}

func RunCardMigration(db *gorm.DB) error {
	var existingMigration models.Migration
	result := db.Where("name = ?", "card_migration").First(&existingMigration)
	if result.Error == nil && existingMigration.ID != 0 {
		log.Println("Card migration has already been executed. Skipping.")
		return nil
	}

	log.Println("Starting card migration...")

	rarities := []models.CardRarity{
		{
			Name:    "Common",
			Weight:  2000,
			Color:   "0x95A5A6",
			Icon:    "🤍",
			Royalty: 0.5,
		},
		{
			Name:    "Uncommon",
			Weight:  1825,
			Color:   "0x2ECC71",
			Icon:    "💚",
			Royalty: 1.0,
		},
		{
			Name:    "Rare",
			Weight:  1250,
			Color:   "0x3498DB",
			Icon:    "💙",
			Royalty: 2.0,
		},
		{
			Name:    "Epic",
			Weight:  365,
			Color:   "0x9B59B6",
			Icon:    "💜",
			Royalty: 5.0,
		},
		{
			Name:    "Mythic",
			Weight:  250,
			Color:   "0xF1C40F",
			Icon:    "✨",
			Royalty: 25.0,
		},
	}

	rarityMap := make(map[string]uint)
	for _, rarity := range rarities {
		var existingRarity models.CardRarity
		result := db.Where("name = ?", rarity.Name).First(&existingRarity)
		if result.Error == nil {
			log.Printf("Rarity %s already exists, using existing", rarity.Name)
			rarityMap[rarity.Name] = existingRarity.ID
			continue
		}

		createdRarity := models.CardRarity{
			ID:      rarity.ID,
			Name:    rarity.Name,
			Weight:  rarity.Weight,
			Color:   rarity.Color,
			Icon:    rarity.Icon,
			Royalty: rarity.Royalty,
		}
		if err := db.Create(&createdRarity).Error; err != nil {
			log.Printf("Error creating rarity %s: %v", rarity.Name, err)
			continue
		}
		rarityMap[rarity.Name] = createdRarity.ID
	}

	var deck []models.Card
	cards.RegisterAllCards(&deck)

	extractHandlerName := func(handler models.CardHandler) string {
		if handler == nil {
			return ""
		}
		funcValue := reflect.ValueOf(handler)
		if !funcValue.IsValid() || funcValue.IsNil() {
			return ""
		}
		funcPtr := funcValue.Pointer()
		if funcPtr == 0 {
			return ""
		}
		fn := runtime.FuncForPC(funcPtr)
		if fn == nil {
			return ""
		}
		fullName := fn.Name()
		// Extract just the handler name (e.g., "handleGambler" from "perfectOddsBot/services/cardService/cards.handleGambler")
		parts := strings.Split(fullName, ".")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return fullName
	}

	successCount := 0
	for _, card := range deck {
		cardOptions := card.Options

		card.HandlerName = extractHandlerName(card.Handler)

		if rarityID, exists := rarityMap[card.Rarity]; exists {
			card.RarityID = rarityID
		} else {
			log.Printf("Warning: Rarity '%s' not found in map for card %d (%s)", card.Rarity, card.ID, card.Name)
			continue
		}

		card.Active = true

		var existingCard models.Card
		result := db.Where("id = ?", card.ID).First(&existingCard)
		if result.Error == nil {
			log.Printf("Card %d (%s) already exists, skipping creation", card.ID, card.Name)
			successCount++
			if len(cardOptions) > 0 {
				for _, option := range cardOptions {
					var existingOption models.CardOption
					optionResult := db.Where("id = ?", option.ID).First(&existingOption)
					if optionResult.Error == nil {
						log.Printf("Card option %d already exists, skipping", option.ID)
						continue
					}
					cardOption := models.CardOption{
						ID:          option.ID,
						CardID:      card.ID,
						Name:        option.Name,
						Description: option.Description,
					}
					if err := db.Create(&cardOption).Error; err != nil {
						log.Printf("Error creating option %d for card %d: %v", option.ID, card.ID, err)
						continue
					}
				}
			}
			continue
		}

		if err := db.Create(&card).Error; err != nil {
			log.Printf("Error creating card %d (%s): %v", card.ID, card.Name, err)
			continue
		}

		successCount++

		if len(cardOptions) > 0 {
			for _, option := range cardOptions {
				var existingOption models.CardOption
				optionResult := db.Where("id = ?", option.ID).First(&existingOption)
				if optionResult.Error == nil {
					log.Printf("Card option %d already exists, skipping", option.ID)
					continue
				}
				cardOption := models.CardOption{
					ID:          option.ID,
					CardID:      card.ID,
					Name:        option.Name,
					Description: option.Description,
				}
				if err := db.Create(&cardOption).Error; err != nil {
					log.Printf("Error creating option %d for card %d: %v", option.ID, card.ID, err)
					continue
				}
			}
		}
	}
	log.Printf("Created %d cards (out of %d total)", successCount, len(deck))

	migration := models.Migration{
		Name:       "card_migration",
		ExecutedAt: time.Now(),
	}
	if err := db.Create(&migration).Error; err != nil {
		return fmt.Errorf("error marking migration as complete: %v", err)
	}

	log.Println("Card migration completed.")
	return nil
}

// SyncCards compares cards in code with cards in the database and updates the database if changes are detected.
// This function runs on every startup to ensure the database stays in sync with code changes.
func SyncCards(db *gorm.DB) error {
	log.Println("Starting card sync from code...")

	rarityMap := make(map[string]uint)
	var rarities []models.CardRarity
	if err := db.Find(&rarities).Error; err != nil {
		return fmt.Errorf("error fetching rarities: %v", err)
	}
	for _, rarity := range rarities {
		rarityMap[rarity.Name] = rarity.ID
	}

	var codeDeck []models.Card
	cards.RegisterAllCards(&codeDeck)

	extractHandlerName := func(handler models.CardHandler) string {
		if handler == nil {
			return ""
		}
		funcValue := reflect.ValueOf(handler)
		if !funcValue.IsValid() || funcValue.IsNil() {
			return ""
		}
		funcPtr := funcValue.Pointer()
		if funcPtr == 0 {
			return ""
		}
		fn := runtime.FuncForPC(funcPtr)
		if fn == nil {
			return ""
		}
		fullName := fn.Name()
		parts := strings.Split(fullName, ".")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return fullName
	}

	updatedCount := 0
	createdCount := 0
	optionsUpdatedCount := 0

	for _, codeCard := range codeDeck {
		codeCardOptions := codeCard.Options
		codeCard.HandlerName = extractHandlerName(codeCard.Handler)

		var dbCard models.Card
		result := db.Where("id = ?", codeCard.ID).First(&dbCard)

		if result.Error != nil {
			cardToCreate := codeCard
			cardToCreate.Options = []models.CardOption{}
			// set rarity id to common by default since one is required to be set
			cardToCreate.RarityID = 1
			cardToCreate.Weight = 0
			cardToCreate.Rarity = ""
			cardToCreate.HandlerName = codeCard.HandlerName
			cardToCreate.AddToInventory = codeCard.AddToInventory
			cardToCreate.RequiredSubscription = codeCard.RequiredSubscription
			cardToCreate.UserPlayable = codeCard.UserPlayable
			cardToCreate.RoyaltyDiscordUserID = codeCard.RoyaltyDiscordUserID

			if err := db.Create(&cardToCreate).Error; err != nil {
				log.Printf("Error creating card %d (%s): %v", codeCard.ID, codeCard.Name, err)
				continue
			}

			// Explicitly set Active to false after creation to override database default
			if err := db.Model(&cardToCreate).Update("active", false).Error; err != nil {
				log.Printf("Error setting Active=false for card %d (%s): %v", codeCard.ID, codeCard.Name, err)
				continue
			}
			createdCount++
			log.Printf("Created new card %d (%s) with Active=false (configure in database to activate)", codeCard.ID, codeCard.Name)

			if len(codeCardOptions) > 0 {
				for _, option := range codeCardOptions {
					cardOption := models.CardOption{
						ID:          option.ID,
						CardID:      codeCard.ID,
						Name:        option.Name,
						Description: option.Description,
					}
					if err := db.Create(&cardOption).Error; err != nil {
						log.Printf("Error creating option %d for card %d: %v", option.ID, codeCard.ID, err)
					} else {
						optionsUpdatedCount++
						log.Printf("Created new option %d (%s) for card %d", option.ID, option.Name, codeCard.ID)
					}
				}
			}
			continue
		}

		needsUpdate := false
		updateFields := make(map[string]interface{})

		if dbCard.Code != codeCard.Code {
			updateFields["code"] = codeCard.Code
			needsUpdate = true
		}
		if dbCard.Name != codeCard.Name {
			updateFields["name"] = codeCard.Name
			needsUpdate = true
		}
		if dbCard.Description != codeCard.Description {
			updateFields["description"] = codeCard.Description
			needsUpdate = true
		}
		if dbCard.HandlerName != codeCard.HandlerName {
			updateFields["handler_name"] = codeCard.HandlerName
			needsUpdate = true
		}
		if dbCard.AddToInventory != codeCard.AddToInventory {
			updateFields["add_to_inventory"] = codeCard.AddToInventory
			needsUpdate = true
		}
		if (dbCard.RoyaltyDiscordUserID == nil && codeCard.RoyaltyDiscordUserID != nil) ||
			(dbCard.RoyaltyDiscordUserID != nil && codeCard.RoyaltyDiscordUserID == nil) ||
			(dbCard.RoyaltyDiscordUserID != nil && codeCard.RoyaltyDiscordUserID != nil && *dbCard.RoyaltyDiscordUserID != *codeCard.RoyaltyDiscordUserID) {
			updateFields["royalty_discord_user_id"] = codeCard.RoyaltyDiscordUserID
			needsUpdate = true
		}
		if dbCard.RequiredSubscription != codeCard.RequiredSubscription {
			updateFields["required_subscription"] = codeCard.RequiredSubscription
			needsUpdate = true
		}
		if dbCard.UserPlayable != codeCard.UserPlayable {
			updateFields["user_playable"] = codeCard.UserPlayable
			needsUpdate = true
		}

		if needsUpdate {
			if err := db.Model(&dbCard).Updates(updateFields).Error; err != nil {
				log.Printf("Error updating card %d (%s): %v", codeCard.ID, codeCard.Name, err)
				continue
			}
			updatedCount++
			log.Printf("Updated card %d (%s) with fields: %v", codeCard.ID, codeCard.Name, updateFields)
		}

		if len(codeCardOptions) > 0 {
			var dbOptions []models.CardOption
			if err := db.Where("card_id = ?", codeCard.ID).Find(&dbOptions).Error; err != nil {
				log.Printf("Error fetching options for card %d: %v", codeCard.ID, err)
			} else {
				dbOptionsMap := make(map[uint]models.CardOption)
				for _, opt := range dbOptions {
					dbOptionsMap[opt.ID] = opt
				}

				codeOptionsMap := make(map[uint]bool)
				for _, codeOption := range codeCardOptions {
					codeOptionsMap[codeOption.ID] = true

					var existingOption models.CardOption
					optionResult := db.Where("id = ?", codeOption.ID).First(&existingOption)

					if optionResult.Error != nil {
						cardOption := models.CardOption{
							ID:          codeOption.ID,
							CardID:      codeCard.ID,
							Name:        codeOption.Name,
							Description: codeOption.Description,
						}
						if err := db.Create(&cardOption).Error; err != nil {
							log.Printf("Error creating option %d for card %d: %v", codeOption.ID, codeCard.ID, err)
						} else {
							optionsUpdatedCount++
							log.Printf("Created new option %d (%s) for card %d", codeOption.ID, codeOption.Name, codeCard.ID)
						}
					} else {
						optionNeedsUpdate := false
						optionUpdateFields := make(map[string]interface{})

						if existingOption.Name != codeOption.Name {
							optionUpdateFields["name"] = codeOption.Name
							optionNeedsUpdate = true
						}
						if existingOption.Description != codeOption.Description {
							optionUpdateFields["description"] = codeOption.Description
							optionNeedsUpdate = true
						}
						if existingOption.CardID != codeCard.ID {
							optionUpdateFields["card_id"] = codeCard.ID
							optionNeedsUpdate = true
						}

						if optionNeedsUpdate {
							if err := db.Model(&existingOption).Updates(optionUpdateFields).Error; err != nil {
								log.Printf("Error updating option %d for card %d: %v", codeOption.ID, codeCard.ID, err)
							} else {
								optionsUpdatedCount++
								log.Printf("Updated option %d (%s) for card %d", codeOption.ID, codeOption.Name, codeCard.ID)
							}
						}
					}
				}
			}
		}
	}

	log.Printf("Card sync completed. Created: %d, Updated: %d, Options updated: %d (out of %d total cards)", createdCount, updatedCount, optionsUpdatedCount, len(codeDeck))
	return nil
}
