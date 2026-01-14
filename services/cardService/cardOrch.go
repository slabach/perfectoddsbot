package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

// DrawCard handles the /draw-card command
func DrawCard(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	// Get guild info
	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	// Check if card drawing is enabled
	if !guild.CardDrawingEnabled {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Card drawing is currently disabled for this server.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	// Get or create user
	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: userID, GuildID: guildID})
	if result.Error != nil {
		common.SendError(s, i, fmt.Errorf("error fetching user: %v", result.Error), db)
		return
	}
	if result.RowsAffected == 1 {
		user.Points = guild.StartingPoints
		db.Save(&user)
	}

	// Update username
	username := common.GetUsernameFromUser(i.Member.User)
	common.UpdateUserUsername(db, &user, username)

	// Check if user is timed out from drawing cards
	now := time.Now()
	if user.CardDrawTimeoutUntil != nil && now.Before(*user.CardDrawTimeoutUntil) {
		timeRemaining := user.CardDrawTimeoutUntil.Sub(now)
		minutesRemaining := int(timeRemaining.Minutes())
		secondsRemaining := int(timeRemaining.Seconds()) % 60
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("You are timed out from drawing cards. Time remaining: %d minutes and %d seconds.", minutesRemaining, secondsRemaining),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	// Clear timeout if it has expired
	if user.CardDrawTimeoutUntil != nil && now.After(*user.CardDrawTimeoutUntil) {
		user.CardDrawTimeoutUntil = nil
	}

	// Get reset period from guild
	resetPeriod := time.Duration(guild.CardDrawCooldownMinutes) * time.Minute

	// Check if reset period has passed, reset if needed
	if user.FirstCardDrawCycle != nil {
		timeSinceFirstDraw := now.Sub(*user.FirstCardDrawCycle)
		if timeSinceFirstDraw >= resetPeriod {
			// Reset cycle
			user.FirstCardDrawCycle = &now
			user.CardDrawCount = 0
		}
	} else {
		// First draw ever - start cycle
		user.FirstCardDrawCycle = &now
		user.CardDrawCount = 0
	}

	// Calculate progressive cost: 10, 100, 1000, 1000, ...
	var drawCardCost float64
	switch user.CardDrawCount {
	case 0:
		drawCardCost = guild.CardDrawCost
	case 1:
		drawCardCost = guild.CardDrawCost * 10
	default:
		drawCardCost = guild.CardDrawCost * 100
	}

	// Check for Generous Donation (if cost is standard/level 1)
	var donorUserID uint
	var donorName string
	if drawCardCost == guild.CardDrawCost {
		donorID, err := hasGenerousDonationInInventory(db, guildID)
		if err != nil {
			common.SendError(s, i, fmt.Errorf("error checking donation inventory: %v", err), db)
			return
		}

		// If found and donor is NOT the current user
		if donorID != 0 && donorID != user.ID {
			donorUserID = donorID
			// Get donor name for display
			var donor models.User
			if err := db.First(&donor, donorID).Error; err == nil {
				donorName = common.GetUsernameWithDB(db, s, guildID, donor.DiscordID)
			}

			// Reduce cost to 0 for this user
			drawCardCost = 0
		}
	}

	// Check for Lucky Horseshoe (read-only check before transaction)
	hasLuckyHorseshoe, err := hasLuckyHorseshoeInInventory(db, user.ID, guildID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error checking inventory: %v", err), db)
		return
	}
	if hasLuckyHorseshoe {
		drawCardCost = drawCardCost * 0.5
	}

	// Check for Unlucky Cat (read-only check before transaction)
	hasUnluckyCat, err := hasUnluckyCatInInventory(db, user.ID, guildID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error checking inventory: %v", err), db)
		return
	}
	if hasUnluckyCat {
		drawCardCost = drawCardCost * 2.0
	}

	// Check if user has enough points
	if user.Points < drawCardCost {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("You need at least %.0f points to draw a card. You have %.1f points.", drawCardCost, user.Points),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	// Start transaction for atomic operations
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Consume Lucky Horseshoe if user has one (inside transaction)
	if hasLuckyHorseshoe {
		if err := PlayCardFromInventory(s, tx, user, cards.LuckyHorseshoeCardID); err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error consuming Lucky Horseshoe: %v", err), db)
			return
		}
	}

	// Consume Unlucky Cat if user has one (inside transaction)
	if hasUnluckyCat {
		if err := PlayCardFromInventory(s, tx, user, cards.UnluckyCatCardID); err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error consuming Unlucky Cat: %v", err), db)
			return
		}
	}

	// Consume Generous Donation if applied
	if donorUserID != 0 {
		var donorUser models.User
		if err := tx.First(&donorUser, donorUserID).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error fetching donor user: %v", err), db)
			return
		}

		if err := PlayCardFromInventory(s, tx, donorUser, cards.GenerousDonationCardID); err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error consuming Generous Donation: %v", err), db)
			return
		}
	}

	// Deduct cost
	user.Points -= drawCardCost

	// Add to pool
	guild.Pool += drawCardCost

	// Increment draw count
	user.CardDrawCount++

	// Save user changes (cost deducted, count incremented) before handler
	// This ensures handler can update user and we can reload it with all changes
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return
	}

	// Pick random card
	// Check if guild has a subscribed team
	hasSubscription := guild.SubscribedTeam != nil && *guild.SubscribedTeam != ""
	card := PickRandomCard(hasSubscription)
	if card == nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("no cards available"), db)
		return
	}

	// Process royalty payment if card has a royalty user
	if err := processRoyaltyPayment(tx, card, cards.RoyaltyGuildID); err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error processing royalty payment: %v", err), db)
		return
	}

	// Add card to inventory if it should be added
	if card.AddToInventory {
		if err := addCardToInventory(tx, user.ID, guildID, card.ID); err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error adding card to inventory: %v", err), db)
			return
		}
	}

	// Execute card handler (pass tx so handler updates are part of transaction)
	cardResult, err := card.Handler(s, tx, userID, guildID)
	if err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error executing card effect: %v", err), db)
		return
	}

	// If card requires user selection (e.g., Pickpocket), handle it specially
	if cardResult.RequiresSelection {
		// Save intermediate state - we need to store the card ID and user state
		// For now, we'll handle Pickpocket selection immediately
		if cardResult.SelectionType == "user" {
			// Show user select menu
			showUserSelectMenu(s, i, card.ID, card.Name, card.Description, userID, guildID, db)

			// Reload user to get any handler updates (e.g., timeout)
			if err := tx.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}

			// Apply card effects
			user.Points += cardResult.PointsDelta
			// Ensure points never go below 0
			if user.Points < 0 {
				user.Points = 0
			}
			guild.Pool += cardResult.PoolDelta

			// Save partial state (cost deducted, pool updated, cooldown set)
			if err := tx.Save(&user).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}
			if err := tx.Save(&guild).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}

			tx.Commit()
			return
		}
	}

	// Reload user from transaction to get any updates made by handler (e.g., timeout)
	if err := tx.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error reloading user: %v", err), db)
		return
	}

	// Apply card effects
	// If the user has a Shield and this is a negative effect, block it
	if cardResult.PointsDelta < 0 {
		hasShield, err := hasShieldInInventory(tx, user.ID, guildID)
		if err != nil {
			tx.Rollback()
			common.SendError(s, i, err, db)
			return
		}
		if hasShield {
			if err := PlayCardFromInventory(s, tx, user, cards.ShieldCardID); err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}

			cardResult.PointsDelta = 0
			if cardResult.Message == "" {
				cardResult.Message = "Your Shield blocked the hit!"
			} else {
				cardResult.Message += " (Your Shield blocked the hit!)"
			}
		}
	}

	user.Points += cardResult.PointsDelta
	// Ensure points never go below 0
	if user.Points < 0 {
		user.Points = 0
	}
	guild.Pool += cardResult.PoolDelta

	// Update target user if applicable
	var targetUsername string
	if cardResult.TargetUserID != nil {
		var targetUser models.User
		if err := tx.Where("discord_id = ? AND guild_id = ?", *cardResult.TargetUserID, guildID).First(&targetUser).Error; err == nil {
			// If the target has a Shield and this is a negative effect, block it
			if cardResult.TargetPointsDelta < 0 {
				hasShield, err := hasShieldInInventory(tx, targetUser.ID, guildID)
				if err != nil {
					tx.Rollback()
					common.SendError(s, i, err, db)
					return
				}
				if hasShield {
					if err := PlayCardFromInventory(s, tx, targetUser, cards.ShieldCardID); err != nil {
						tx.Rollback()
						common.SendError(s, i, err, db)
						return
					}

					cardResult.TargetPointsDelta = 0
					if cardResult.Message == "" {
						cardResult.Message = "Their Shield blocked the hit!"
					} else {
						cardResult.Message += " (Their Shield blocked the hit!)"
					}
				}
			}

			targetUser.Points += cardResult.TargetPointsDelta
			if targetUser.Points < 0 {
				targetUser.Points = 0
			}
			tx.Save(&targetUser)
			targetUsername = common.GetUsernameWithDB(db, s, guildID, *cardResult.TargetUserID)
		}
	}

	// Save all changes
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return
	}
	if err := tx.Save(&guild).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return
	}

	tx.Commit()

	// Build embed response
	embed := buildCardEmbed(card, cardResult, user, targetUsername, guild.Pool, drawCardCost)

	// If Generous Donation was used, append to footer
	if donorUserID != 0 && donorName != "" {
		if embed.Footer == nil {
			embed.Footer = &discordgo.MessageEmbedFooter{}
		}
		originalText := embed.Footer.Text
		embed.Footer.Text = fmt.Sprintf("%s | Paid for by generous donation from %s!", originalText, donorName)
	}

	// Special handling for Rick Roll card - add YouTube link to content for auto-preview
	var content string
	if card.ID == cards.RickRollCardID { // Rick Roll card ID
		content = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Embeds:  []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
}

// showUserSelectMenu displays a user select menu for cards that require target selection
func showUserSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, cardID int, cardName string, cardDescription string, userID string, guildID string, db *gorm.DB) {
	minValues := 1
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 You drew **%s**!\n%s\n\nSelect a user to target:", cardName, cardDescription),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.UserSelectMenu,
							CustomID:    fmt.Sprintf("card_%d_select_%s_%s", cardID, userID, guildID),
							Placeholder: "Choose a user...",
							MinValues:   &minValues,
							MaxValues:   1,
						},
					},
				},
			},
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
	}
}

// buildCardEmbed creates a rich embed for the card draw result
func buildCardEmbed(card *models.Card, result *models.CardResult, user models.User, targetUsername string, poolBalance float64, drawCardCost float64) *discordgo.MessageEmbed {
	// Determine rarity color
	var color int
	switch card.Rarity {
	case "Common":
		color = cards.C_Common // Gray
	case "Uncommon":
		color = cards.C_Uncommon // Green
	case "Rare":
		color = cards.C_Rare // Blue
	case "Epic":
		color = cards.C_Epic // Purple
	case "Mythic":
		color = cards.C_Mythic // Gold
	default:
		color = cards.C_Common // Gray
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎴 You Drew: %s", card.Name),
		Description: card.Description,
		Color:       color,
		Fields:      []*discordgo.MessageEmbedField{},
	}

	// Add rarity field
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Rarity",
		Value:  card.Rarity,
		Inline: true,
	})

	// Add result message
	if result.Message != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Effect",
			Value:  result.Message,
			Inline: false,
		})
	}

	// Add points delta for user
	if result.PointsDelta != 0 {
		sign := "+"
		if result.PointsDelta < 0 {
			sign = ""
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Points Change",
			Value:  fmt.Sprintf("<@%s>: %s%.1f points (Total: %.1f)", user.DiscordID, sign, result.PointsDelta, user.Points),
			Inline: true,
		})
	}

	// Add target user points delta if applicable
	if result.TargetUserID != nil && result.TargetPointsDelta != 0 {
		sign := "+"
		if result.TargetPointsDelta < 0 {
			sign = ""
		}
		// We need to calculate the target's new total (this will be passed in if needed)
		// For now, just show the change
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Target Change",
			Value:  fmt.Sprintf("<@%s>: %s%.1f points", *result.TargetUserID, sign, result.TargetPointsDelta),
			Inline: true,
		})
	}

	// Add pool balance
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Pool Balance",
		Value:  fmt.Sprintf("%.1f points", poolBalance),
		Inline: true,
	})

	// Add cost info
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Cost: -%.0f points | Added %.0f to pool", drawCardCost, drawCardCost),
	}

	return embed
}

// hasLuckyHorseshoeInInventory checks if user has a Lucky Horseshoe in inventory (read-only)
func hasLuckyHorseshoeInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.LuckyHorseshoeCardID).
		Count(&count).Error
	return count > 0, err
}

// hasUnluckyCatInInventory checks if user has an Unlucky Cat in inventory (read-only)
func hasUnluckyCatInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.UnluckyCatCardID).
		Count(&count).Error
	return count > 0, err
}

// hasShieldInInventory checks if user has a Shield in inventory (read-only)
func hasShieldInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.ShieldCardID).
		Count(&count).Error
	return count > 0, err
}

// hasGenerousDonationInInventory checks if ANY user in the guild has a Generous Donation card
// Returns the user ID of the first donor found, or 0 if none
func hasGenerousDonationInInventory(db *gorm.DB, guildID string) (uint, error) {
	var inventory models.UserInventory
	// Find ANY user in this guild who has the card
	err := db.Model(&models.UserInventory{}).
		Where("guild_id = ? AND card_id = ?", guildID, cards.GenerousDonationCardID).
		First(&inventory).Error

	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return inventory.UserID, nil
}

// CardConsumer is a function type for consuming a card from inventory
type CardConsumer func(db *gorm.DB, user models.User, cardID int) error

// ApplyDoubleDownIfAvailable checks if user has Double Down card and applies 2x multiplier to payout
// Returns the modified payout and whether Double Down was applied
func ApplyDoubleDownIfAvailable(db *gorm.DB, consumer CardConsumer, user models.User, originalPayout float64) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.DoubleDownCardID).
		Count(&count).Error

	if err != nil {
		return originalPayout, false, err
	}

	if count > 0 {
		// User has Double Down - consume it and double the payout
		if err := consumer(db, user, cards.DoubleDownCardID); err != nil {
			return originalPayout, false, err
		}
		return originalPayout * 2.0, true, nil
	}

	return originalPayout, false, nil
}

// ApplyEmotionalHedgeIfApplicable checks if user has Emotional Hedge card and applies refund if conditions met
// Returns the refund amount (if any) and whether the card was applied (consumed)
func ApplyEmotionalHedgeIfApplicable(db *gorm.DB, consumer CardConsumer, user models.User, bet models.Bet, userPick int, betAmount float64, scoreDiff int) (float64, bool, error) {
	// 1. Check if user has the card
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.EmotionalHedgeCardID).
		Count(&count).Error
	if err != nil {
		return 0, false, err
	}
	if count == 0 {
		return 0, false, nil
	}

	// 2. Check if guild has a subscribed team
	var guild models.Guild
	if err := db.Where("guild_id = ?", user.GuildID).First(&guild).Error; err != nil {
		return 0, false, err
	}
	if guild.SubscribedTeam == nil || *guild.SubscribedTeam == "" {
		return 0, false, nil
	}
	subscribedTeam := *guild.SubscribedTeam

	// 3. Check if user's pick is the subscribed team
	var userPickedTeamName string
	if userPick == 1 {
		userPickedTeamName = bet.Option1
	} else {
		userPickedTeamName = bet.Option2
	}

	userPickedTeamNameNormalized := common.GetSchoolName(userPickedTeamName)
	subscribedTeamNormalized := common.GetSchoolName(subscribedTeam)

	isBetOnSubscribedTeam := userPickedTeamNameNormalized == subscribedTeamNormalized

	if !isBetOnSubscribedTeam {
		// Try looser check in case normalization misses
		isBetOnSubscribedTeam = (userPickedTeamName == subscribedTeam)
	}

	if !isBetOnSubscribedTeam {
		return 0, false, nil
	}

	// 4. Consume the card
	if err := consumer(db, user, cards.EmotionalHedgeCardID); err != nil {
		return 0, false, err
	}

	// 5. Check if the subscribed team lost STRAIGHT UP
	// scoreDiff is (Option1Score - Option2Score).
	// If userPick == 1, team won if scoreDiff > 0.
	// If userPick == 2, team won if scoreDiff < 0.

	teamWonStraightUp := false
	if userPick == 1 {
		teamWonStraightUp = scoreDiff > 0
	} else {
		teamWonStraightUp = scoreDiff < 0
	}

	if !teamWonStraightUp {
		// Team lost straight up -> Refund 50%
		refund := betAmount * 0.5
		return refund, true, nil
	}

	return 0, true, nil
}

// processRoyaltyPayment handles royalty payments to card creators when their cards are drawn
func processRoyaltyPayment(tx *gorm.DB, card *models.Card, royaltyGuildID string) error {
	// Check if card has a royalty user
	if card.RoyaltyDiscordUserID == nil {
		return nil // No royalty to pay
	}

	// Calculate royalty amount based on rarity
	var royaltyAmount float64
	switch card.Rarity {
	case "Common":
		royaltyAmount = 0.5
	case "Uncommon":
		royaltyAmount = 1.0
	case "Rare":
		royaltyAmount = 2.0
	case "Epic":
		royaltyAmount = 5.0
	case "Mythic":
		royaltyAmount = 25.0
	default:
		// Unknown rarity, default to common
		royaltyAmount = 0.5
	}

	// Get or create guild info for the royalty guild to set starting points if needed
	var royaltyGuild models.Guild
	guildResult := tx.Where("guild_id = ?", royaltyGuildID).First(&royaltyGuild)
	if guildResult.Error != nil {
		return fmt.Errorf("error fetching royalty guild: %v", guildResult.Error)
	}

	// Get or create royalty user in the specific guild
	var royaltyUser models.User
	result := tx.First(&royaltyUser, models.User{
		DiscordID: *card.RoyaltyDiscordUserID,
		GuildID:   royaltyGuildID,
	})
	if result.Error != nil {
		return fmt.Errorf("error fetching royalty user: %v", result.Error)
	}

	// Add royalty amount to user's points
	royaltyUser.Points += royaltyAmount

	// Save the royalty user
	if err := tx.Save(&royaltyUser).Error; err != nil {
		return fmt.Errorf("error saving royalty user: %v", err)
	}

	return nil
}

// addCardToInventory adds a card to the user's inventory
func addCardToInventory(db *gorm.DB, userID uint, guildID string, cardID int) error {
	inventory := models.UserInventory{
		UserID:  userID,
		GuildID: guildID,
		CardID:  cardID,
	}
	return db.Create(&inventory).Error
}

// getUserInventory gets all active cards in a user's inventory
func getUserInventory(db *gorm.DB, userID uint, guildID string) ([]models.UserInventory, error) {
	var inventory []models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ?", userID, guildID).Find(&inventory).Error
	return inventory, err
}

// MyInventory handles the /my-inventory command
func MyInventory(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	// Get or create user
	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: userID, GuildID: guildID, Points: guild.StartingPoints})
	if result.Error != nil {
		common.SendError(s, i, fmt.Errorf("error fetching user: %v", result.Error), db)
		return
	}

	// Get user's inventory
	inventory, err := getUserInventory(db, user.ID, guildID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error fetching inventory: %v", err), db)
		return
	}

	// Group inventory by card ID and count quantities
	cardCounts := make(map[int]int)
	for _, item := range inventory {
		cardCounts[item.CardID]++
	}

	// If inventory is empty
	if len(cardCounts) == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Your inventory is empty. Draw some cards to add them to your hand!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	// Organize cards by rarity
	rarityOrder := []string{"Mythic", "Epic", "Rare", "Common"}
	cardsByRarity := make(map[string][]struct {
		Card  *models.Card
		Count int
	})

	for cardID, count := range cardCounts {
		card := GetCardByID(cardID)
		if card == nil {
			continue // Skip if card not found
		}
		if cardsByRarity[card.Rarity] == nil {
			cardsByRarity[card.Rarity] = []struct {
				Card  *models.Card
				Count int
			}{}
		}
		cardsByRarity[card.Rarity] = append(cardsByRarity[card.Rarity], struct {
			Card  *models.Card
			Count int
		}{Card: card, Count: count})
	}

	// Build embed
	embed := &discordgo.MessageEmbed{
		Title:       "🎴 Your Inventory",
		Description: "Cards currently in your hand",
		Color:       0x3498DB, // Blue
		Fields:      []*discordgo.MessageEmbedField{},
	}

	// Add cards organized by rarity
	for _, rarity := range rarityOrder {
		cards, exists := cardsByRarity[rarity]
		if !exists || len(cards) == 0 {
			continue
		}

		// Build field value for this rarity
		var fieldValue string
		for _, cardInfo := range cards {
			quantityText := ""
			if cardInfo.Count > 1 {
				quantityText = fmt.Sprintf(" (x%d)", cardInfo.Count)
			}
			fieldValue += fmt.Sprintf("**%s**%s\n%s\n\n", cardInfo.Card.Name, quantityText, cardInfo.Card.Description)
		}

		// Determine rarity color/emoji
		var rarityEmoji string
		switch rarity {
		case "Mythic":
			rarityEmoji = "✨"
		case "Epic":
			rarityEmoji = "💜"
		case "Rare":
			rarityEmoji = "💙"
		case "Common":
			rarityEmoji = "⚪"
		default:
			rarityEmoji = "📄"
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s", rarityEmoji, rarity),
			Value:  fieldValue,
			Inline: false,
		})
	}

	// Add footer with total count
	totalCards := 0
	for _, count := range cardCounts {
		totalCards += count
	}
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Total cards: %d", totalCards),
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
}
