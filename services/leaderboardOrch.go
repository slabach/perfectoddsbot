package services

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
)

func ShowLeaderboard(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	guildID := i.GuildID

	var users []models.User
	db.Where("guild_id = ?", guildID).Order("points desc").Limit(10).Find(&users)

	if len(users) == 0 {
		response := "No users found on the leaderboard."
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
			},
		})
		if err != nil {
			return
		}
		return
	}

	description := ""
	for idx, user := range users {
		username := common.GetUsername(s, user.GuildID, user.DiscordID)

		description += fmt.Sprintf("**%d. %s** - %.1f points\n", idx+1, username, user.Points)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🏆 Leaderboard",
		Description: description,
		Color:       0x00ff00,
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		return
	}
}
