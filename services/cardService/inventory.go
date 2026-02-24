package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/guildService"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func PlayCardFromInventory(s *discordgo.Session, db *gorm.DB, user models.User, cardID uint) error {
	return PlayCardFromInventoryWithMessage(s, db, user, cardID, "")
}

func PlayCardFromInventoryInTransaction(tx *gorm.DB, user models.User, cardID uint) error {
	var inventory models.UserInventory
	result := tx.Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cardID).First(&inventory)

	if result.Error != nil {
		return result.Error
	}

	return tx.Delete(&inventory).Error
}

func PlayCardFromInventoryWithMessage(s *discordgo.Session, db *gorm.DB, user models.User, cardID uint, customMessage string) error {
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
	if card.CardRarity.ID != 0 {
		colorStr := card.CardRarity.Color
		if len(colorStr) > 2 && colorStr[0:2] == "0x" {
			colorStr = colorStr[2:]
		}
		_, err := fmt.Sscanf(colorStr, "%x", &color)
		if err != nil {
			color = 0x95A5A6
		}
	} else {
		color = 0x95A5A6
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
