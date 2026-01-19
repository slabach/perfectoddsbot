package cardService

import (
	"fmt"
	"math/rand"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CardConsumer func(db *gorm.DB, user models.User, cardID int) error

func DrawCard(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

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

	username := common.GetUsernameFromUser(i.Member.User)
	common.UpdateUserUsername(db, &user, username)

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

	if user.CardDrawTimeoutUntil != nil && now.After(*user.CardDrawTimeoutUntil) {
		user.CardDrawTimeoutUntil = nil
	}

	resetPeriod := time.Duration(guild.CardDrawCooldownMinutes) * time.Minute

	// Track whether a reset is needed before we lock the user
	shouldResetCount := false
	if user.FirstCardDrawCycle != nil {
		timeSinceFirstDraw := now.Sub(*user.FirstCardDrawCycle)
		if timeSinceFirstDraw >= resetPeriod {
			user.FirstCardDrawCycle = &now
			user.CardDrawCount = 0
			shouldResetCount = true
		}
	} else {
		user.FirstCardDrawCycle = &now
		user.CardDrawCount = 0
		shouldResetCount = true
	}

	// Start transaction early to perform inventory checks with row locking
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Compute base drawCardCost (before modifiers)
	var drawCardCost float64
	switch user.CardDrawCount {
	case 0:
		drawCardCost = guild.CardDrawCost
	case 1:
		drawCardCost = guild.CardDrawCost * 10
	default:
		drawCardCost = guild.CardDrawCost * 100
	}

	var donorUserID uint
	var donorName string
	if drawCardCost == guild.CardDrawCost {
		donorID, err := hasGenerousDonationInInventory(db, guildID)
		if err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error checking donation inventory: %v", err), db)
			return
		}

		if donorID != 0 && donorID != user.ID {
			donorUserID = donorID
			var donor models.User
			if err := db.First(&donor, donorID).Error; err == nil {
				donorName = common.GetUsernameWithDB(db, s, guildID, donor.DiscordID)
			}

			drawCardCost = 0
		}
	}

	var horseshoeInventory models.UserInventory
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, guildID, cards.LuckyHorseshoeCardID).
		First(&horseshoeInventory).Error
	hasLuckyHorseshoe := err == nil
	if err != nil && err != gorm.ErrRecordNotFound {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error checking Lucky Horseshoe inventory: %v", err), db)
		return
	}
	if hasLuckyHorseshoe {
		drawCardCost = drawCardCost * 0.5
	}

	var unluckyCatInventory models.UserInventory
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, guildID, cards.UnluckyCatCardID).
		First(&unluckyCatInventory).Error
	hasUnluckyCat := err == nil
	if err != nil && err != gorm.ErrRecordNotFound {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error checking Unlucky Cat inventory: %v", err), db)
		return
	}
	if hasUnluckyCat {
		drawCardCost = drawCardCost * 2.0
	}

	var lockedUser models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedUser, user.ID).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error locking user: %v", err), db)
		return
	}

	if lockedUser.Points < drawCardCost {
		tx.Rollback()
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("You need at least %.0f points to draw a card. You have %.1f points.", drawCardCost, lockedUser.Points),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	// Consume cards atomically in the same transaction
	if hasLuckyHorseshoe {
		if err := tx.Delete(&horseshoeInventory).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error consuming Lucky Horseshoe: %v", err), db)
			return
		}
	}

	if hasUnluckyCat {
		if err := tx.Delete(&unluckyCatInventory).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error consuming Unlucky Cat: %v", err), db)
			return
		}
	}

	user = lockedUser

	// Re-apply the reset logic if it was needed, since lockedUser has the old DB state
	if shouldResetCount {
		user.CardDrawCount = 0
		user.FirstCardDrawCycle = &now
	}

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

	user.Points -= drawCardCost
	guild.Pool += drawCardCost

	user.CardDrawCount++

	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return
	}

	hasSubscription := guild.SubscribedTeam != nil && *guild.SubscribedTeam != ""
	card := PickRandomCard(hasSubscription)
	if card == nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("no cards available"), db)
		return
	}

	if err := processRoyaltyPayment(tx, card, cards.RoyaltyGuildID); err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error processing royalty payment: %v", err), db)
		return
	}

	// Add card to inventory if AddToInventory flag is set OR if it's UserPlayable
	if card.AddToInventory || card.UserPlayable {
		if err := addCardToInventory(tx, user.ID, guildID, card.ID); err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error adding card to inventory: %v", err), db)
			return
		}
	}

	cardResult, err := card.Handler(s, tx, userID, guildID)
	if err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error executing card effect: %v", err), db)
		return
	}

	if cardResult.RequiresSelection {
		if cardResult.SelectionType == "user" {
			// Hostile Takeover requires filtered user selection (within 500 points)
			if card.ID == cards.HostileTakeoverCardID {
				showFilteredUserSelectMenu(s, i, card.ID, card.Name, card.Description, userID, guildID, db, 500.0)
			} else {
				showUserSelectMenu(s, i, card.ID, card.Name, card.Description, userID, guildID, db)
			}

			if err := tx.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}

			user.Points += cardResult.PointsDelta
			if user.Points < 0 {
				user.Points = 0
			}
			guild.Pool += cardResult.PoolDelta

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
		} else if cardResult.SelectionType == "bet" {
			showBetSelectMenu(s, i, card.ID, card.Name, card.Description, userID, guildID, db)

			if err := tx.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}

			user.Points += cardResult.PointsDelta
			if user.Points < 0 {
				user.Points = 0
			}
			guild.Pool += cardResult.PoolDelta

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

	// Check if card has Options (for choice cards like The Gambler)
	if len(card.Options) > 0 {
		showCardOptionsMenu(s, i, card.ID, card.Name, card.Description, userID, guildID, db, card.Options)

		// Update user points from cardResult if needed
		if err := tx.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, err, db)
			return
		}

		user.Points += cardResult.PointsDelta
		if user.Points < 0 {
			user.Points = 0
		}
		guild.Pool += cardResult.PoolDelta

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

	if err := tx.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error reloading user: %v", err), db)
		return
	}

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
	if user.Points < 0 {
		user.Points = 0
	}
	guild.Pool += cardResult.PoolDelta

	var targetUsername string
	if cardResult.TargetUserID != nil {
		var targetUser models.User
		if err := tx.Where("discord_id = ? AND guild_id = ?", *cardResult.TargetUserID, guildID).First(&targetUser).Error; err == nil {
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

	embed := buildCardEmbed(card, cardResult, user, targetUsername, guild.Pool, drawCardCost)

	if donorUserID != 0 && donorName != "" {
		if embed.Footer == nil {
			embed.Footer = &discordgo.MessageEmbedFooter{}
		}
		originalText := embed.Footer.Text
		embed.Footer.Text = fmt.Sprintf("%s | Paid for by generous donation from %s!", originalText, donorName)
	}

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

func showFilteredUserSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, cardID int, cardName string, cardDescription string, userID string, guildID string, db *gorm.DB, maxPointDifference float64) {
	var drawer models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&drawer).Error; err != nil {
		common.SendError(s, i, err, db)
		return
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ?", guildID, userID).Find(&allUsers).Error; err != nil {
		common.SendError(s, i, err, db)
		return
	}

	var eligibleUsers []models.User
	for _, u := range allUsers {
		pointDiff := drawer.Points - u.Points
		if pointDiff < 0 {
			pointDiff = -pointDiff
		}
		if pointDiff <= maxPointDifference {
			eligibleUsers = append(eligibleUsers, u)
		}
	}

	if len(eligibleUsers) == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("🎴 You drew **%s**!\n%s\n\nNo users found within %.0f points of you (you have %.1f points). Hostile Takeover fizzles out.", cardName, cardDescription, maxPointDifference, drawer.Points),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	var selectOptions []discordgo.SelectMenuOption
	for _, u := range eligibleUsers {
		username := u.Username
		displayName := ""
		if username == nil || *username == "" {
			displayName = fmt.Sprintf("User %s", u.DiscordID)
		} else {
			displayName = *username
		}

		// Truncate if too long (Discord limit is 100 chars for label, 100 for description)
		if len(displayName) > 100 {
			displayName = displayName[:97] + "..."
		}

		description := fmt.Sprintf("%.1f points", u.Points)
		if len(description) > 100 {
			description = description[:97] + "..."
		}

		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label:       displayName,
			Value:       u.DiscordID,
			Description: description,
			Emoji:       nil,
			Default:     false,
		})
	}

	minValues := 1
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 You drew **%s**!\n%s\n\nSelect a user within %.0f points of you (you have %.1f points):", cardName, cardDescription, maxPointDifference, drawer.Points),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("card_%d_select_%s_%s", cardID, userID, guildID),
							Placeholder: "Choose a user...",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     selectOptions,
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

func showBetSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, cardID int, cardName string, cardDescription string, userID string, guildID string, db *gorm.DB) {
	var results []struct {
		BetID       uint
		Description string
		Option      int
		Option1     string
		Option2     string
	}

	err := db.Table("bet_entries").
		Select("bets.id as bet_id, bets.description, bet_entries.option, bets.option1, bets.option2").
		Joins("JOIN bets ON bets.id = bet_entries.bet_id").
		Where("bet_entries.user_id = (SELECT id FROM users WHERE discord_id = ? AND guild_id = ?) AND bets.paid = ?", userID, guildID, false).
		Limit(25).
		Scan(&results).Error

	if err != nil {
		common.SendError(s, i, fmt.Errorf("error fetching bets: %v", err), db)
		return
	}

	if len(results) == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("🎴 You drew **%s**!\n%s\n\nYou have no active bets to use this card on! The card fizzles out.", cardName, cardDescription),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	options := []discordgo.SelectMenuOption{}
	for _, res := range results {
		pickedTeam := res.Option1
		if res.Option == 2 {
			pickedTeam = res.Option2
		}

		label := fmt.Sprintf("%s (Pick: %s)", res.Description, pickedTeam)
		if len(label) > 100 {
			label = label[:97] + "..."
		}

		options = append(options, discordgo.SelectMenuOption{
			Label: label,
			Value: fmt.Sprintf("%d", res.BetID),
		})
	}

	minValues := 1
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 You drew **%s**!\n%s\n\nSelect an active bet to target:", cardName, cardDescription),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("card_%d_selectbet_%s_%s", cardID, userID, guildID),
							Placeholder: "Choose a bet...",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     options,
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

func showCardOptionsMenu(s *discordgo.Session, i *discordgo.InteractionCreate, cardID int, cardName string, cardDescription string, userID string, guildID string, db *gorm.DB, options []models.CardOption) {
	var selectOptions []discordgo.SelectMenuOption
	for _, opt := range options {
		label := opt.Name
		description := opt.Description

		// Truncate if too long (Discord limit is 100 chars for label, 100 for description)
		if len(label) > 100 {
			label = label[:97] + "..."
		}
		if len(description) > 100 {
			description = description[:97] + "..."
		}

		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label:       label,
			Value:       fmt.Sprintf("%d", opt.ID),
			Description: description,
			Emoji:       nil,
			Default:     false,
		})
	}

	// Build options list string for display in message
	var optionsList strings.Builder
	for i, opt := range options {
		if i > 0 {
			optionsList.WriteString("\n")
		}
		optionsList.WriteString(fmt.Sprintf("**%s**: %s", opt.Name, opt.Description))
	}

	minValues := 1
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 You drew **%s**!\n%s\n\n%s\n\nSelect an option:", cardName, cardDescription, optionsList.String()),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("card_%d_option_%s_%s", cardID, userID, guildID),
							Placeholder: "Choose an option...",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     selectOptions,
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

func buildCardEmbed(card *models.Card, result *models.CardResult, user models.User, targetUsername string, poolBalance float64, drawCardCost float64) *discordgo.MessageEmbed {
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

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Rarity",
		Value:  card.Rarity,
		Inline: true,
	})

	if result.Message != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Effect",
			Value:  result.Message,
			Inline: false,
		})
	}

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

	if result.TargetUserID != nil && result.TargetPointsDelta != 0 {
		sign := "+"
		if result.TargetPointsDelta < 0 {
			sign = ""
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Target Change",
			Value:  fmt.Sprintf("<@%s>: %s%.1f points", *result.TargetUserID, sign, result.TargetPointsDelta),
			Inline: true,
		})
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Pool Balance",
		Value:  fmt.Sprintf("%.1f points", poolBalance),
		Inline: true,
	})

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Cost: -%.0f points | Added %.0f to pool", drawCardCost, drawCardCost),
	}

	return embed
}

func hasLuckyHorseshoeInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.LuckyHorseshoeCardID).
		Count(&count).Error
	return count > 0, err
}

func hasUnluckyCatInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.UnluckyCatCardID).
		Count(&count).Error
	return count > 0, err
}

func hasShieldInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.ShieldCardID).
		Count(&count).Error
	return count > 0, err
}

func hasGenerousDonationInInventory(db *gorm.DB, guildID string) (uint, error) {
	var inventory models.UserInventory
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

func ApplyUnoReverseIfApplicable(db *gorm.DB, user models.User, betID uint, originalIsWin bool) (bool, bool, error) {
	var inventory models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND target_bet_id = ?", user.ID, user.GuildID, cards.UnoReverseCardID, betID).
		First(&inventory).Error

	if err == gorm.ErrRecordNotFound {
		return false, originalIsWin, nil
	}
	if err != nil {
		return false, originalIsWin, err
	}

	if err := db.Delete(&inventory).Error; err != nil {
		return false, originalIsWin, err
	}

	return true, !originalIsWin, nil
}

type AntiAntiBetWinner struct {
	DiscordID string
	Payout    float64
}

func ApplyAntiAntiBetIfApplicable(db *gorm.DB, bettorUser models.User, isWin bool) (totalPayout float64, winners []AntiAntiBetWinner, losers []AntiAntiBetWinner, applied bool, err error) {
	var userCards []models.UserInventory
	err = db.Where("guild_id = ? AND card_id = ? AND target_user_id = ?", bettorUser.GuildID, cards.AntiAntiBetCardID, bettorUser.DiscordID).
		Find(&userCards).Error

	if err != nil {
		return 0, nil, nil, false, err
	}

	if len(userCards) == 0 {
		return 0, nil, nil, false, nil
	}

	totalPayout = 0.0
	applied = true
	winners = []AntiAntiBetWinner{}
	losers = []AntiAntiBetWinner{}

	for _, card := range userCards {
		if isWin {
			// Bet won: card holder loses, just delete the card
			var cardHolder models.User
			if err := db.First(&cardHolder, card.UserID).Error; err != nil {
				// If we can't find the user, still delete the card
				db.Delete(&card)
				continue
			}

			// Add card holder to losers list
			losers = append(losers, AntiAntiBetWinner{
				DiscordID: cardHolder.DiscordID,
				Payout:    card.BetAmount, // The amount they bet (which they lose)
			})

			if err := db.Delete(&card).Error; err != nil {
				return totalPayout, winners, losers, applied, err
			}
		} else {
			payout := common.CalculateSimplePayout(card.BetAmount)

			var cardHolder models.User
			if err := db.First(&cardHolder, card.UserID).Error; err != nil {
				db.Delete(&card)
				continue
			}

			cardHolder.Points += payout
			if err := db.Model(&cardHolder).UpdateColumn("points", gorm.Expr("points + ?", payout)).Error; err != nil {
				return totalPayout, winners, losers, applied, err
			}

			totalPayout += payout
			winners = append(winners, AntiAntiBetWinner{
				DiscordID: cardHolder.DiscordID,
				Payout:    payout,
			})

			if err := db.Delete(&card).Error; err != nil {
				return totalPayout, winners, losers, applied, err
			}
		}
	}

	return totalPayout, winners, losers, applied, nil
}

type VampireWinner struct {
	DiscordID string
	Payout    float64
}

func ApplyVampireIfApplicable(db *gorm.DB, guildID string, totalWinningPayouts float64, winnerDiscordIDs map[string]float64) (totalVampirePayout float64, winners []VampireWinner, applied bool, err error) {
	twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)
	var vampireCards []models.UserInventory
	err = db.Where("guild_id = ? AND card_id = ? AND created_at >= ?", guildID, cards.VampireCardID, twentyFourHoursAgo).
		Find(&vampireCards).Error

	if err != nil {
		return 0, nil, false, err
	}

	if len(vampireCards) == 0 {
		return 0, nil, false, nil
	}

	totalVampirePayout = 0.0
	applied = true
	winners = []VampireWinner{}

	for _, card := range vampireCards {
		var vampireHolder models.User
		if err := db.First(&vampireHolder, card.UserID).Error; err != nil {
			continue
		}

		vampireHolderWinnings := 0.0
		if winnerDiscordIDs != nil {
			vampireHolderWinnings = winnerDiscordIDs[vampireHolder.DiscordID]
		}
		totalOtherWinnings := totalWinningPayouts - vampireHolderWinnings

		if totalOtherWinnings < 0 {
			continue
		}
		if totalWinningPayouts > 0 && totalOtherWinnings == 0 {
			continue
		}

		vampirePayout := totalOtherWinnings * 0.01

		if vampirePayout > 500.0 {
			vampirePayout = 500.0
		}

		if err := db.Model(&vampireHolder).UpdateColumn("points", gorm.Expr("points + ?", vampirePayout)).Error; err != nil {
			return totalVampirePayout, winners, applied, err
		}

		totalVampirePayout += vampirePayout
		winners = append(winners, VampireWinner{
			DiscordID: vampireHolder.DiscordID,
			Payout:    vampirePayout,
		})
	}

	return totalVampirePayout, winners, applied, nil
}

func ApplyDoubleDownIfAvailable(db *gorm.DB, consumer CardConsumer, user models.User, originalPayout float64) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.DoubleDownCardID).
		Count(&count).Error

	if err != nil {
		return originalPayout, false, err
	}

	if count > 0 {
		if err := consumer(db, user, cards.DoubleDownCardID); err != nil {
			return originalPayout, false, err
		}
		return originalPayout * 2.0, true, nil
	}

	return originalPayout, false, nil
}

func ApplyEmotionalHedgeIfApplicable(db *gorm.DB, consumer CardConsumer, user models.User, bet models.Bet, userPick int, betAmount float64, scoreDiff int) (float64, bool, error) {
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

	var guild models.Guild
	if err := db.Where("guild_id = ?", user.GuildID).First(&guild).Error; err != nil {
		return 0, false, err
	}
	if guild.SubscribedTeam == nil || *guild.SubscribedTeam == "" {
		return 0, false, nil
	}
	subscribedTeam := *guild.SubscribedTeam

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
		isBetOnSubscribedTeam = (userPickedTeamName == subscribedTeam)
	}

	if !isBetOnSubscribedTeam {
		return 0, false, nil
	}

	if scoreDiff == 0 {
		return 0, true, nil
	}

	teamWonStraightUp := false
	if userPick == 1 {
		teamWonStraightUp = scoreDiff > 0
	} else {
		teamWonStraightUp = scoreDiff < 0
	}

	if !teamWonStraightUp {
		if err := consumer(db, user, cards.EmotionalHedgeCardID); err != nil {
			return 0, false, err
		}
		refund := betAmount * 0.5
		return refund, true, nil
	}

	return 0, true, nil
}

func ApplyBetInsuranceIfApplicable(db *gorm.DB, consumer CardConsumer, user models.User, betAmount float64, isWin bool) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.BetInsuranceCardID).
		Count(&count).Error

	if err != nil {
		return 0, false, err
	}

	if count > 0 {
		if err := consumer(db, user, cards.BetInsuranceCardID); err != nil {
			return 0, false, err
		}

		if !isWin {
			refund := betAmount * 0.25
			return refund, true, nil
		} else {
			return 0, true, nil
		}
	}

	return 0, false, nil
}

func ApplyGetOutOfJailIfApplicable(db *gorm.DB, consumer CardConsumer, user models.User, betAmount float64) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.GetOutOfJailCardID).
		Count(&count).Error

	if err != nil {
		return 0, false, err
	}

	if count > 0 {
		if err := consumer(db, user, cards.GetOutOfJailCardID); err != nil {
			return 0, false, err
		}

		return betAmount, true, nil
	}

	return 0, false, nil
}

func ApplyGamblerIfAvailable(db *gorm.DB, consumer CardConsumer, user models.User, originalPayout float64, isWin bool) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.GamblerCardID).
		Count(&count).Error

	if err != nil {
		return originalPayout, false, err
	}

	if count > 0 {
		// Consume the card regardless of whether doubling occurs
		if err := consumer(db, user, cards.GamblerCardID); err != nil {
			return originalPayout, false, err
		}

		if rand.Intn(2) == 0 {
			return originalPayout * 2.0, true, nil
		}

		return originalPayout, true, nil
	}

	return originalPayout, false, nil
}

func processRoyaltyPayment(tx *gorm.DB, card *models.Card, royaltyGuildID string) error {
	if card.RoyaltyDiscordUserID == nil {
		return nil
	}

	var royaltyAmount float64
	switch card.Rarity {
	case "Common":
		royaltyAmount = cards.R_Common
	case "Uncommon":
		royaltyAmount = cards.R_Uncommon
	case "Rare":
		royaltyAmount = cards.R_Rare
	case "Epic":
		royaltyAmount = cards.R_Epic
	case "Mythic":
		royaltyAmount = cards.R_Mythic
	default:
		royaltyAmount = cards.R_Common
	}

	var royaltyGuild models.Guild
	guildResult := tx.Where("guild_id = ?", royaltyGuildID).First(&royaltyGuild)
	if guildResult.Error != nil {
		return fmt.Errorf("error fetching royalty guild: %v", guildResult.Error)
	}

	var royaltyUser models.User
	result := tx.First(&royaltyUser, models.User{
		DiscordID: *card.RoyaltyDiscordUserID,
		GuildID:   royaltyGuildID,
	})
	if result.Error != nil {
		return fmt.Errorf("error fetching royalty user: %v", result.Error)
	}

	if err := tx.Model(&royaltyUser).UpdateColumn("points", gorm.Expr("points + ?", royaltyAmount)).Error; err != nil {
		return fmt.Errorf("error saving royalty user: %v", err)
	}

	return nil
}

func addCardToInventory(db *gorm.DB, userID uint, guildID string, cardID int) error {
	inventory := models.UserInventory{
		UserID:  userID,
		GuildID: guildID,
		CardID:  cardID,
	}
	return db.Create(&inventory).Error
}

func getUserInventory(db *gorm.DB, userID uint, guildID string) ([]models.UserInventory, error) {
	var inventory []models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ?", userID, guildID).Find(&inventory).Error
	return inventory, err
}

func MyInventory(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	var user models.User
	result := db.Where(models.User{DiscordID: userID, GuildID: guildID}).Attrs(models.User{Points: guild.StartingPoints}).FirstOrCreate(&user)
	if result.Error != nil {
		common.SendError(s, i, fmt.Errorf("error fetching user: %v", result.Error), db)
		return
	}

	inventory, err := getUserInventory(db, user.ID, guildID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error fetching inventory: %v", err), db)
		return
	}

	cardCounts := make(map[int]int)
	for _, item := range inventory {
		cardCounts[item.CardID]++
	}

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

	rarityOrder := []string{"Mythic", "Epic", "Rare", "Uncommon", "Common"}
	cardsByRarity := make(map[string][]struct {
		Card  *models.Card
		Count int
	})

	for cardID, count := range cardCounts {
		card := GetCardByID(cardID)
		if card == nil {
			continue
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

	embed := &discordgo.MessageEmbed{
		Title:       "🎴 Your Inventory",
		Description: "Cards currently in your hand",
		Color:       0x3498DB, // Blue
		Fields:      []*discordgo.MessageEmbedField{},
	}

	for _, rarity := range rarityOrder {
		cardsHeld, exists := cardsByRarity[rarity]
		if !exists || len(cardsHeld) == 0 {
			continue
		}

		var fieldValue string
		for _, cardInfo := range cardsHeld {
			quantityText := ""
			if cardInfo.Count > 1 {
				quantityText = fmt.Sprintf(" (x%d)", cardInfo.Count)
			}
			fieldValue += fmt.Sprintf("**%s**%s\n%s\n\n", cardInfo.Card.Name, quantityText, cardInfo.Card.Description)
		}

		var rarityEmoji string
		switch rarity {
		case "Mythic":
			rarityEmoji = cards.E_Mythic
		case "Epic":
			rarityEmoji = cards.E_Epic
		case "Rare":
			rarityEmoji = cards.E_Rare
		case "Uncommon":
			rarityEmoji = cards.E_Uncommon
		case "Common":
			rarityEmoji = cards.E_Common
		default:
			rarityEmoji = cards.E_Common
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

	// Calculate countdown until timer resets and next draw cost
	now := time.Now()
	resetPeriod := time.Duration(guild.CardDrawCooldownMinutes) * time.Minute
	
	var countdownText string
	if user.FirstCardDrawCycle != nil {
		resetTime := user.FirstCardDrawCycle.Add(resetPeriod)
		if now.Before(resetTime) {
			timeRemaining := resetTime.Sub(now)
			hours := int(timeRemaining.Hours())
			minutes := int(timeRemaining.Minutes()) % 60
			seconds := int(timeRemaining.Seconds()) % 60
			if hours > 0 {
				countdownText = fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
			} else if minutes > 0 {
				countdownText = fmt.Sprintf("%dm %ds", minutes, seconds)
			} else {
				countdownText = fmt.Sprintf("%ds", seconds)
			}
		} else {
			countdownText = "Ready to reset"
		}
	} else {
		countdownText = "No draws yet"
	}

	// Calculate next draw cost
	// If timer has reset, the next draw count will be 0
	var nextDrawCount int
	if user.FirstCardDrawCycle != nil {
		resetTime := user.FirstCardDrawCycle.Add(resetPeriod)
		if now.After(resetTime) || now.Equal(resetTime) {
			// Timer has reset, next draw will be count 0
			nextDrawCount = 0
		} else {
			// Timer hasn't reset yet, use current count
			nextDrawCount = user.CardDrawCount
		}
	} else {
		// No draws yet, next will be count 0
		nextDrawCount = 0
	}

	var nextDrawCost float64
	switch nextDrawCount {
	case 0:
		nextDrawCost = guild.CardDrawCost
	case 1:
		nextDrawCost = guild.CardDrawCost * 10
	default:
		nextDrawCost = guild.CardDrawCost * 100
	}

	// Check for Generous Donation (only affects first draw)
	if nextDrawCount == 0 {
		donorID, err := hasGenerousDonationInInventory(db, guildID)
		if err == nil && donorID != 0 && donorID != user.ID {
			nextDrawCost = 0
		}
	}

	// Check for modifiers (applied sequentially like in DrawCard)
	hasLuckyHorseshoe := cardCounts[cards.LuckyHorseshoeCardID] > 0
	if hasLuckyHorseshoe {
		nextDrawCost = nextDrawCost * 0.5
	}

	hasUnluckyCat := cardCounts[cards.UnluckyCatCardID] > 0
	if hasUnluckyCat {
		nextDrawCost = nextDrawCost * 2.0
	}

	// Add card draw info fields
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "⏱️ Timer Reset",
		Value:  countdownText,
		Inline: true,
	})
	
	costText := fmt.Sprintf("%.0f points", nextDrawCost)
	if nextDrawCost == 0 {
		costText = "Free (Generous Donation)"
	}
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "💰 Next Draw Cost",
		Value:  costText,
		Inline: true,
	})

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

func PlayCard(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	var user models.User
	result := db.Where(models.User{DiscordID: userID, GuildID: guildID}).Attrs(models.User{Points: guild.StartingPoints}).FirstOrCreate(&user)
	if result.Error != nil {
		common.SendError(s, i, fmt.Errorf("error fetching user: %v", result.Error), db)
		return
	}

	showPlayableCardSelectMenu(s, i, db, userID, guildID, user.ID)
}

func showPlayableCardSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, guildID string, userDBID uint) {
	inventory, err := getUserInventory(db, userDBID, guildID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error fetching inventory: %v", err), db)
		return
	}

	// Build a map of cardID -> count for inventory
	inventoryMap := make(map[int]int)
	for _, item := range inventory {
		inventoryMap[item.CardID]++
	}

	// Find all UserPlayable cards in inventory
	var playableCards []struct {
		Card  *models.Card
		Count int
	}

	for cardID, count := range inventoryMap {
		card := GetCardByID(cardID)
		if card != nil && card.UserPlayable {
			playableCards = append(playableCards, struct {
				Card  *models.Card
				Count int
			}{Card: card, Count: count})
		}
	}

	if len(playableCards) == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You don't have any playable cards in your inventory. Draw some cards to get started!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	// Limit to 25 options (Discord select menu limit)
	maxOptions := 25
	if len(playableCards) > maxOptions {
		playableCards = playableCards[:maxOptions]
	}

	var selectOptions []discordgo.SelectMenuOption
	for _, pc := range playableCards {
		label := pc.Card.Name
		if pc.Count > 1 {
			label = fmt.Sprintf("%s (x%d)", pc.Card.Name, pc.Count)
		}

		// Truncate if too long (Discord limit is 100 chars for label, 100 for description)
		if len(label) > 100 {
			label = label[:97] + "..."
		}

		description := pc.Card.Description
		if len(description) > 100 {
			description = description[:97] + "..."
		}

		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label:       label,
			Value:       fmt.Sprintf("%d", pc.Card.ID),
			Description: description,
			Emoji:       nil,
			Default:     false,
		})
	}

	minValues := 1
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Select a card to play from your inventory:",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("playcard_select_%s_%s", userID, guildID),
							Placeholder: "Choose a card to play...",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     selectOptions,
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
