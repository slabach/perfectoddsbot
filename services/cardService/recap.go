package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func ShowRecap(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	_, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	// Default to 7 days
	days := 7
	
	// Check if user provided options
	options := i.ApplicationCommandData().Options
	for _, opt := range options {
		if opt.Name == "days" {
			days = int(opt.IntValue())
		}
	}

	if days < 1 {
		days = 1
	}
	if days > 30 {
		days = 30
	}

	startTime := time.Now().AddDate(0, 0, -days)

	var history []models.CardPlayHistory
	if err := db.Where("target_user_id = ? AND guild_id = ? AND created_at >= ?", userID, guildID, startTime).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		common.SendError(s, i, fmt.Errorf("error fetching history: %v", err), db)
		return
	}

	if len(history) == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("No card history found for you in the last %d days.", days),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📜 Card History (Last %d Days)", days),
		Description: fmt.Sprintf("Here is a recap of card effects on you since %s.", startTime.Format("Jan 02")),
		Color:       0x9B59B6,
		Fields:      []*discordgo.MessageEmbedField{},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Ordered from newest to oldest",
		},
	}

	const maxFields = 25
	count := 0

	for _, h := range history {
		if count >= maxFields {
			embed.Footer.Text += fmt.Sprintf(" | Showing first %d of %d events", maxFields, len(history))
			break
		}

		timestamp := fmt.Sprintf("<t:%d:R>", h.CreatedAt.Unix())
		
		var actionText string
		if h.PlayedByUserID == userID {
			actionText = "You played"
		} else {
			actionText = fmt.Sprintf("<@%s> played", h.PlayedByUserID)
		}

		value := fmt.Sprintf("%s **%s**\n", actionText, h.CardName)
		
		if h.PointsDelta != 0 {
			sign := "+"
			if h.PointsDelta < 0 {
				sign = ""
			}
			value += fmt.Sprintf("Points: %s%.0f (%.0f → %.0f)\n", sign, h.PointsDelta, h.PointsBefore, h.PointsAfter)
		}

		if h.HandCardsGained != "" && h.HandCardsGained != "null" {
			value += fmt.Sprintf("Gained cards: %s\n", h.HandCardsGained)
		}
		if h.HandCardsLost != "" && h.HandCardsLost != "null" {
			value += fmt.Sprintf("Lost cards: %s\n", h.HandCardsLost)
		}
		if h.BetsResolved != "" && h.BetsResolved != "null" {
			value += "Bets resolved\n"
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   timestamp,
			Value:  value,
			Inline: false,
		})
		count++
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
	}
}
