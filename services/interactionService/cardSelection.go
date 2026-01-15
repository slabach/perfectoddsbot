package interactionService

import (
	"fmt"
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

	card := cardService.GetCardByID(4)
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
