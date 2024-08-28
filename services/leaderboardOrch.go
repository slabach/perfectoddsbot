package services

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"predictionOddsBot/models"
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
		member, err := s.GuildMember(guildID, user.DiscordID)
		username := "Unknown User"
		if err == nil {
			username = member.User.GlobalName
		}
		description += fmt.Sprintf("**%d. %s** - %d points\n", idx+1, username, user.Points)
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
