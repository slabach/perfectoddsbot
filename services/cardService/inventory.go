package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/guildService"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

// PlayCardFromInventory consumes a card from a user's inventory and announces it
func PlayCardFromInventory(s *discordgo.Session, db *gorm.DB, user models.User, cardID int) error {
	// 1. Verify & Consume
	var inventory models.UserInventory
	// Using the provided db (which should be a transaction if called from within one)
	result := db.Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cardID).First(&inventory)

	if result.Error != nil {
		return result.Error // Card not found or other DB error
	}

	// Soft delete the card (consume it)
	if err := db.Delete(&inventory).Error; err != nil {
		return err
	}

	// 2. Get Details
	card := GetCardByID(cardID)
	if card == nil {
		return fmt.Errorf("card definition not found for ID %d", cardID)
	}

	// 3. Get Channel
	guild, err := guildService.GetGuildInfo(s, db, user.GuildID, "")
	if err != nil {
		return fmt.Errorf("error getting guild info: %v", err)
	}

	if guild.BetChannelID == "" {
		// No betting channel set, skip notification
		return nil
	}

	// 4. Notify
	// Determine color based on rarity
	var color int
	switch card.Rarity {
	case "Common":
		color = 0x95A5A6 // Gray
	case "Rare":
		color = 0x3498DB // Blue
	case "Epic":
		color = 0x9B59B6 // Purple
	case "Mythic":
		color = 0xF1C40F // Gold
	default:
		color = 0x95A5A6
	}

	description := fmt.Sprintf("<@%s> played **%s**", user.DiscordID, card.Name)

	embed := &discordgo.MessageEmbed{
		Title:       "🎴 Card Played!",
		Description: description,
		Color:       color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Effect",
				Value:  card.Description,
				Inline: false,
			},
		},
	}

	_, err = s.ChannelMessageSendEmbed(guild.BetChannelID, embed)
	if err != nil {
		// Log error but don't fail operation
		fmt.Printf("Error sending card played notification: %v\n", err)
	}

	return nil
}
