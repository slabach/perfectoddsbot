package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func ParseHexColor(colorStr string) int {
	if colorStr == "" {
		return 0x95A5A6
	}
	colorStrLower := strings.ToLower(colorStr)
	if len(colorStrLower) > 2 && colorStrLower[0:2] == "0x" {
		colorStr = colorStr[2:]
	}
	var color int
	_, err := fmt.Sscanf(colorStr, "%x", &color)
	if err != nil {
		return 0x95A5A6
	}
	return color
}

func buildCardEmbed(card *models.Card, result *models.CardResult, user models.User, username string, targetUsername string, poolBalance float64, drawCardCost float64) *discordgo.MessageEmbed {
	var color int
	if card.CardRarity.ID != 0 {
		color = ParseHexColor(card.CardRarity.Color)
	} else {
		color = 0x95A5A6 // Default to Common color
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎴 %s Drew: %s", username, card.Name),
		Description: card.Description,
		Color:       color,
		Fields:      []*discordgo.MessageEmbedField{},
	}

	rarityName := "Common"
	if card.CardRarity.ID != 0 {
		rarityName = card.CardRarity.Name
	}
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Rarity",
		Value:  rarityName,
		Inline: true,
	})

	if result.Message != "" {
		effectValue := result.Message
		const discordFieldValueLimit = 1024
		if len(effectValue) > discordFieldValueLimit {
			effectValue = effectValue[:discordFieldValueLimit-3] + "..."
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Effect",
			Value:  effectValue,
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
