package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/guildService"

	"perfectOddsBot/services/cardService/cards"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func PlayCardFromInventory(s *discordgo.Session, db *gorm.DB, user models.User, cardID int) error {
	return PlayCardFromInventoryWithMessage(s, db, user, cardID, "")
}

func PlayCardFromInventoryWithMessage(s *discordgo.Session, db *gorm.DB, user models.User, cardID int, customMessage string) error {
	card := GetCardByID(cardID)
	if card == nil {
		return fmt.Errorf("card definition not found for ID %d", cardID)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		var inventory models.UserInventory
		result := tx.Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cardID).First(&inventory)

		if result.Error != nil {
			return result.Error
		}

		return tx.Delete(&inventory).Error
	}); err != nil {
		return err
	}
	return NotifyCardPlayedWithMessage(s, db, user, card, customMessage)
}

func NotifyCardPlayed(s *discordgo.Session, db *gorm.DB, user models.User, card *models.Card) error {
	return NotifyCardPlayedWithMessage(s, db, user, card, "")
}

func NotifyCardPlayedWithMessage(s *discordgo.Session, db *gorm.DB, user models.User, card *models.Card, customMessage string) error {
	guild, err := guildService.GetGuildInfo(s, db, user.GuildID, "")
	if err != nil {
		return fmt.Errorf("error getting guild info: %v", err)
	}

	if guild.BetChannelID == "" {
		return nil
	}
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

	description := fmt.Sprintf("<@%s> played **%s**", user.DiscordID, card.Name)
	if customMessage != "" {
		description = customMessage
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🎴 Card Played!",
		Description: description,
		Color:       color,
	}

	// Only add Effect field if no custom message (since custom message usually contains the effect info)
	if customMessage == "" {
		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Effect",
				Value:  card.Description,
				Inline: false,
			},
		}
	}

	_, err = s.ChannelMessageSendEmbed(guild.BetChannelID, embed)
	if err != nil {
		fmt.Printf("Error sending card played notification: %v\n", err)
	}

	return nil
}
