package interactionService

import (
	"fmt"
	"math"
	"perfectOddsBot/models"
	cardService "perfectOddsBot/services/cardService"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func HandleCardUserSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	customID := i.MessageComponentData().CustomID

	parts := strings.Split(customID, "_")
	if len(parts) != 5 {
		return fmt.Errorf("invalid card selection custom ID format")
	}

	cardID, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}

	userID := parts[3]
	guildID := parts[4]

	if i.Member.User.ID != userID {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only select a target for your own card draw.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no user selected")
	}
	targetUserID := i.MessageComponentData().Values[0]

	if targetUserID == userID {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You cannot target yourself!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	switch cardID {
	case cards.PettyTheftCardID:
		return handlePettyTheftSelection(s, i, db, userID, targetUserID, guildID)
	case cards.JesterCardID:
		return handleJesterSelection(s, i, db, userID, targetUserID, guildID)
	case cards.BetFreezeCardID:
		return handleBetFreezeSelection(s, i, db, userID, targetUserID, guildID)
	case cards.GrandLarcenyCardID:
		return handleGrandLarcenySelection(s, i, db, userID, targetUserID, guildID)
	case cards.AntiAntiBetCardID:
		return handleAntiAntiBetSelection(s, i, db, userID, targetUserID, guildID)
	case cards.HostileTakeoverCardID:
		return handleHostileTakeoverSelection(s, i, db, userID, targetUserID, guildID)
	default:
		return fmt.Errorf("card %d does not support user selection", cardID)
	}
}

func handlePettyTheftSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	result, err := cards.ExecutePickpocketSteal(db, userID, targetUserID, guildID, 50.0)
	if err != nil {
		return err
	}

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}

	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return err
	}

	targetUsername := common.GetUsernameWithDB(db, s, guildID, targetUserID)

	card := cardService.GetCardByID(cards.PettyTheftCardID)
	if card == nil {
		return fmt.Errorf("card not found")
	}

	embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func handleJesterSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	result, err := cards.ExecuteJesterMute(s, db, userID, targetUserID, guildID)
	if err != nil {
		return err
	}

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}

	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return err
	}

	targetUsername := common.GetUsernameWithDB(db, s, guildID, targetUserID)

	card := cardService.GetCardByID(cards.JesterCardID)
	if card == nil {
		return fmt.Errorf("card not found")
	}

	embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	return err
}

func handleBetFreezeSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	result, err := cards.ExecuteBetFreeze(s, db, userID, targetUserID, guildID)
	if err != nil {
		return err
	}

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}

	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return err
	}

	targetUsername := common.GetUsernameWithDB(db, s, guildID, targetUserID)

	card := cardService.GetCardByID(cards.BetFreezeCardID)
	if card == nil {
		return fmt.Errorf("card not found")
	}

	embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	return err
}

func handleGrandLarcenySelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	result, err := cards.ExecutePickpocketSteal(db, userID, targetUserID, guildID, 150.0)
	if err != nil {
		return err
	}

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}

	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return err
	}

	targetUsername := common.GetUsernameWithDB(db, s, guildID, targetUserID)

	card := cardService.GetCardByID(cards.GrandLarcenyCardID)
	if card == nil {
		return fmt.Errorf("card not found")
	}

	if result.PointsDelta > 0 {
		targetName := targetUsername
		if result.TargetUserID != nil {
			targetName = fmt.Sprintf("<@%s>", *result.TargetUserID)
		}
		result.Message = fmt.Sprintf("Grand Larceny successful! You stole %.0f points from %s!", result.PointsDelta, targetName)
	}

	embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func handleAntiAntiBetSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	// Get the user who drew the card
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return err
	}

	// Calculate bet amount: 100 points if user has >= 100 points, otherwise half of current points rounded to nearest whole number
	var betAmount float64
	if user.Points >= 100.0 {
		betAmount = 100.0
	} else {
		betAmount = math.Round(user.Points / 2.0)
	}

	// Deduct the bet amount from user's points
	user.Points -= betAmount
	if user.Points < 0 {
		user.Points = 0
	}

	// Save user points
	if err := db.Save(&user).Error; err != nil {
		return err
	}

	// Add card to inventory with TargetUserID and BetAmount
	inventory := models.UserInventory{
		UserID:       user.ID,
		GuildID:      guildID,
		CardID:       cards.AntiAntiBetCardID,
		TargetUserID: &targetUserID,
		BetAmount:    betAmount,
	}

	if err := db.Create(&inventory).Error; err != nil {
		return err
	}

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}

	targetUsername := common.GetUsernameWithDB(db, s, guildID, targetUserID)

	card := cardService.GetCardByID(cards.AntiAntiBetCardID)
	if card == nil {
		return fmt.Errorf("card not found")
	}

	embed := buildCardResultEmbed(card, &models.CardResult{
		Message:     fmt.Sprintf("Anti-Anti-Bet active! You bet %.0f points that <@%s> will lose their next bet. If they lose, you'll get %.0f points at even odds (+100).", betAmount, targetUserID, betAmount*2),
		PointsDelta: -betAmount,
		PoolDelta:   0,
	}, user, targetUsername, guild.Pool)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	return err
}

func handleHostileTakeoverSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	var drawer models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&drawer).Error; err != nil {
		return err
	}

	var target models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).First(&target).Error; err != nil {
		return err
	}

	pointDiff := drawer.Points - target.Points
	if pointDiff < 0 {
		pointDiff = -pointDiff
	}
	if pointDiff > 500.0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error: Selected user is not within 500 points of you. The takeover cannot be completed.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return err
	}

	drawerOriginalPoints := drawer.Points
	targetOriginalPoints := target.Points

	drawer.Points, target.Points = target.Points, drawer.Points

	if err := db.Save(&drawer).Error; err != nil {
		return err
	}
	if err := db.Save(&target).Error; err != nil {
		return err
	}

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}

	targetUsername := common.GetUsernameWithDB(db, s, guildID, targetUserID)

	card := cardService.GetCardByID(cards.HostileTakeoverCardID)
	if card == nil {
		return fmt.Errorf("card not found")
	}

	embed := buildCardResultEmbed(card, &models.CardResult{
		Message:           fmt.Sprintf("Hostile Takeover successful! You swapped points with %s.", targetUsername),
		PointsDelta:       targetOriginalPoints - drawerOriginalPoints,
		PoolDelta:         0,
		TargetUserID:      &targetUserID,
		TargetPointsDelta: drawerOriginalPoints - targetOriginalPoints,
	}, drawer, targetUsername, guild.Pool)

	embed.Fields = []*discordgo.MessageEmbedField{
		{
			Name:   "You",
			Value:  fmt.Sprintf("<@%s>: %.1f → %.1f points", drawer.DiscordID, drawerOriginalPoints, drawer.Points),
			Inline: true,
		},
		{
			Name:   "Target",
			Value:  fmt.Sprintf("<@%s>: %.1f → %.1f points", target.DiscordID, targetOriginalPoints, target.Points),
			Inline: true,
		},
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	return err
}

func buildCardResultEmbed(card *models.Card, result *models.CardResult, user models.User, targetUsername string, poolBalance float64) *discordgo.MessageEmbed {
	var color int
	switch card.Rarity {
	case "Common":
		color = cards.C_Common
	case "Uncommon":
		color = cards.C_Uncommon
	case "Rare":
		color = cards.C_Rare
	case "Epic":
		color = cards.C_Epic
	case "Mythic":
		color = cards.C_Mythic
	default:
		color = cards.C_Common
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎴 Card Effect: %s", card.Name),
		Description: result.Message,
		Color:       color,
		Fields:      []*discordgo.MessageEmbedField{},
	}

	if result.PointsDelta != 0 {
		sign := "+"
		if result.PointsDelta < 0 {
			sign = ""
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "You",
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
			Name:   "Target",
			Value:  fmt.Sprintf("<@%s>: %s%.1f points", *result.TargetUserID, sign, result.TargetPointsDelta),
			Inline: true,
		})
	}

	return embed
}

func HandleCardBetSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, "_")
	// card_ID_selectbet_UserID_GuildID
	if len(parts) != 5 {
		return fmt.Errorf("invalid card bet selection custom ID format")
	}

	cardID, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}

	userID := parts[3]
	guildID := parts[4]

	if i.Member.User.ID != userID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only make selections for your own card draw.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no bet selected")
	}
	targetBetIDStr := i.MessageComponentData().Values[0]
	targetBetIDVal, err := strconv.ParseUint(targetBetIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid bet ID: %v", err)
	}
	targetBetID := uint(targetBetIDVal)

	// Get user DB ID
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return err
	}

	// Add card to inventory with TargetBetID
	inventory := models.UserInventory{
		UserID:      user.ID,
		GuildID:     guildID,
		CardID:      cardID,
		TargetBetID: &targetBetID,
	}

	if err := db.Create(&inventory).Error; err != nil {
		return err
	}

	// Fetch card info for response
	card := cardService.GetCardByID(cardID)
	if card == nil {
		return fmt.Errorf("card not found")
	}

	// Get bet info for response
	var bet models.Bet
	if err := db.First(&bet, targetBetID).Error; err != nil {
		return fmt.Errorf("bet not found")
	}

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}

	// Build success embed
	embed := buildCardResultEmbed(card, &models.CardResult{
		Message:     fmt.Sprintf("Uno Reverse card active! If you lose on '%s', you win (and vice versa)!", bet.Description),
		PointsDelta: 0,
		PoolDelta:   0,
	}, user, "", guild.Pool)

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

func HandleCardOptionSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, "_")
	// Format: card_ID_option_UserID_GuildID
	if len(parts) != 5 {
		return fmt.Errorf("invalid card option selection custom ID format")
	}

	cardID, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}

	userID := parts[3]
	guildID := parts[4]

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no option selected")
	}
	selectedOptionIDStr := i.MessageComponentData().Values[0]
	selectedOptionID, err := strconv.Atoi(selectedOptionIDStr)
	if err != nil {
		return fmt.Errorf("invalid option ID: %v", err)
	}

	// Get user DB ID
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return err
	}

	card := cardService.GetCardByID(cardID)
	if card == nil {
		return fmt.Errorf("card not found")
	}

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}

	// Option ID 2 is "No", Option ID 1 is "Yes" for The Gambler
	if selectedOptionID == 2 {
		// "No" option - remove card from inventory if it was added
		// Since card was added in DrawCard before handler was called, we need to remove it
		var inventory models.UserInventory
		result := db.Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, guildID, cardID).First(&inventory)
		if result.Error == nil {
			// Card exists in inventory, remove it
			if err := db.Delete(&inventory).Error; err != nil {
				return fmt.Errorf("error removing card from inventory: %v", err)
			}
		}

		embed := buildCardResultEmbed(card, &models.CardResult{
			Message:     "You chose 'No'. The card fizzles out and is not added to your inventory.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, user, "", guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	}

	// "Yes" option - card stays in inventory (already added in DrawCard)
	embed := buildCardResultEmbed(card, &models.CardResult{
		Message:     "You chose 'Yes'! The Gambler has been added to your inventory. Your next bet resolution has a 50/50 chance to double your win or loss.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, user, "", guild.Pool)

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

func HandlePlayCardSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	customID := i.MessageComponentData().CustomID
	// Format: playcard_select_<userID>_<guildID>
	parts := strings.Split(customID, "_")
	if len(parts) != 4 {
		return fmt.Errorf("invalid playcard selection custom ID format")
	}

	userID := parts[2]
	guildID := parts[3]

	if i.Member.User.ID != userID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only play your own cards.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no card selected")
	}

	selectedCardIDStr := i.MessageComponentData().Values[0]
	selectedCardID, err := strconv.Atoi(selectedCardIDStr)
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}

	// Check if card is STOP THE STEAL (card ID 70)
	if selectedCardID == cards.StopTheStealCardID {
		showBetSelectMenuForPlayCard(s, i, db, selectedCardID, userID, guildID)
		return nil
	}

	// For other cards, handle appropriately in the future
	// For now, return an error
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "This card type is not yet supported for manual play.",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func showBetSelectMenuForPlayCard(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, cardID int, userID string, guildID string) {
	var results []struct {
		BetID       uint
		Description string
		Option1     string
		Option2     string
		TotalAmount int
	}

	err := db.Table("bet_entries").
		Select("bets.id as bet_id, bets.description, bets.option1, bets.option2, SUM(bet_entries.amount) as total_amount").
		Joins("JOIN bets ON bets.id = bet_entries.bet_id").
		Where("bet_entries.user_id = (SELECT id FROM users WHERE discord_id = ? AND guild_id = ?) AND bets.paid = 0 AND bets.guild_id = ? AND bet_entries.deleted_at IS NULL", userID, guildID, guildID).
		Group("bets.id, bets.description, bets.option1, bets.option2").
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
				Content: "You don't have any active bets to cancel.",
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
		// Show bet description with amount bet
		label := fmt.Sprintf("%s (%d pts)", res.Description, res.TotalAmount)
		if len(label) > 100 {
			label = label[:97] + "..."
		}

		options = append(options, discordgo.SelectMenuOption{
			Label: label,
			Value: fmt.Sprintf("%d", res.BetID),
		})
	}

	card := cardService.GetCardByID(cardID)
	cardName := "Card"
	if card != nil {
		cardName = card.Name
	}

	minValues := 1
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 Playing **%s**\n\nSelect an active bet to cancel:", cardName),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("playcard_bet_%d_%s_%s", cardID, userID, guildID),
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

func HandlePlayCardBetSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, "_")
	if len(parts) != 5 {
		return fmt.Errorf("invalid playcard bet selection custom ID format")
	}

	cardID, err := strconv.Atoi(parts[2])
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}

	userID := parts[3]
	guildID := parts[4]

	if i.Member.User.ID != userID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only play your own cards.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no bet selected")
	}

	selectedBetIDStr := i.MessageComponentData().Values[0]
	selectedBetID, err := strconv.ParseUint(selectedBetIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid bet ID: %v", err)
	}
	betID := uint(selectedBetID)

	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %v", err)
	}

	var bet models.Bet
	if err := db.First(&bet, "id = ? AND guild_id = ? AND paid = 0 AND deleted_at IS NULL", betID, guildID).Error; err != nil {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This bet is no longer available for cancellation.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	var entries []models.BetEntry
	if err := db.Where("bet_id = ? AND user_id = ? AND deleted_at IS NULL", betID, user.ID).Find(&entries).Error; err != nil {
		return fmt.Errorf("error querying bet entries: %v", err)
	}

	if len(entries) == 0 {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "No bet entries found to cancel.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	refundAmount := 0
	for _, entry := range entries {
		refundAmount += entry.Amount
	}

	user.Points += float64(refundAmount)
	if err := db.Save(&user).Error; err != nil {
		return fmt.Errorf("error refunding points: %v", err)
	}

	result := db.Where("bet_id = ? AND user_id = ? AND deleted_at IS NULL", betID, user.ID).Delete(&models.BetEntry{})
	if result.Error != nil {
		return fmt.Errorf("error soft deleting bet entries: %v", result.Error)
	}

	card := cardService.GetCardByID(cardID)
	if card == nil {
		return fmt.Errorf("card not found: %d", cardID)
	}

	if err := cardService.PlayCardFromInventoryWithMessage(s, db, user, cardID, fmt.Sprintf("<@%s> played **%s** and cancelled bet: **%s**", userID, card.Name, bet.Description)); err != nil {
		return fmt.Errorf("error consuming card: %v", err)
	}

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return fmt.Errorf("error getting guild info: %v", err)
	}

	embed := buildCardResultEmbed(card, &models.CardResult{
		Message:     fmt.Sprintf("You cancelled your bet: **%s** and received a refund of **%d** points.", bet.Description, refundAmount),
		PointsDelta: float64(refundAmount),
		PoolDelta:   0,
	}, user, "", guild.Pool)

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}
