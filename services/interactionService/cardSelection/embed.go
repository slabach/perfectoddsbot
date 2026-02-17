package cardSelection

import (
	"fmt"
	"perfectOddsBot/models"
	cardService "perfectOddsBot/services/cardService"

	"github.com/bwmarrin/discordgo"
)

func BuildCardResultEmbed(card *models.Card, result *models.CardResult, user models.User, targetUsername string, poolBalance float64) *discordgo.MessageEmbed {
	var color int
	if card.CardRarity.ID != 0 {
		color = cardService.ParseHexColor(card.CardRarity.Color)
	} else {
		color = 0x95A5A6
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
