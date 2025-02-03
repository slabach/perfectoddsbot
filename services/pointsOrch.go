package services

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
)

func ShowPoints(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: userID, GuildID: guildID})
	if result.RowsAffected == 1 {
		user.Points = guild.StartingPoints
		db.Save(&user)
	}

	response := fmt.Sprintf("You have **%.1f** points.", user.Points)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return
	}
}

func GivePoints(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	if !common.IsAdmin(s, i) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You are not authorized to use this command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return
		}
		return
	}

	guild, err := guildService.GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	options := i.ApplicationCommandData().Options
	targetUser := options[0].UserValue(s)
	amount := int(options[1].IntValue())
	guildID := i.GuildID

	if amount <= 0 {
		response := "Please enter a valid amount greater than zero."
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return
		}
		return
	}

	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: targetUser.ID, GuildID: guildID})
	if result.RowsAffected == 1 {
		user.Points = guild.StartingPoints
	}

	user.Points += float64(amount)
	db.Save(&user)

	response := fmt.Sprintf("Successfully gave **%d** points to **%s**.", amount, targetUser.Username)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
		},
	})
	if err != nil {
		return
	}
}

func ResetPoints(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	if !common.IsAdmin(s, i) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You are not authorized to use this command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return
		}
		return
	}

	defaultAmount := 1000
	options := i.ApplicationCommandData().Options
	if len(options) > 0 {
		defaultAmount = int(options[0].IntValue())
		if defaultAmount < 0 {
			response := "Reset amount cannot be negative."
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: response,
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				return
			}
			return
		}
	}

	guildID := i.GuildID
	db.Model(&models.User{}).Where("guild_id = ?", guildID).Update("points", defaultAmount)

	response := fmt.Sprintf("All users' points have been reset to **%d**.", defaultAmount)
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
		},
	})
	if err != nil {
		return
	}
}
