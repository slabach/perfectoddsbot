package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/guildService"

	"perfectOddsBot/services/cardService/cards"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

// PlayCardFromInventory consumes a card from a user's inventory and announces it
func PlayCardFromInventory(s *discordgo.Session, db *gorm.DB, user models.User, cardID int) error {
	// 1. Get Details
	card := GetCardByID(cardID)
	if card == nil {
		return fmt.Errorf("card definition not found for ID %d", cardID)
	}

	// 2. Verify & Consume
	if err := db.Transaction(func(tx *gorm.DB) error {
		var inventory models.UserInventory
		// Using the transaction tx
		result := tx.Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cardID).First(&inventory)

		if result.Error != nil {
			return result.Error // Card not found or other DB error
		}

		// Soft delete the card (consume it)
		return tx.Delete(&inventory).Error
	}); err != nil {
		return err
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
