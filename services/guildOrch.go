package services

import (
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
)

func SetBettingChannel(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	if !common.IsAdmin(s, i) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You are not authorized to use this command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}
		return
	}

	var guild models.Guild
	result := db.FirstOrCreate(&guild, models.Guild{GuildID: i.GuildID, BetChannelID: i.ChannelID})
	if result.RowsAffected == 1 {
		guild.BetChannelID = i.ChannelID
		db.Save(&guild)
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Channel set successfully",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
}
