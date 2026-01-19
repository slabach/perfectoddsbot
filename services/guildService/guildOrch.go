package guildService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func GetGuildInfo(s *discordgo.Session, db *gorm.DB, guildID string, channelId string) (*models.Guild, error) {
	var guild models.Guild
	guildResult := db.Where("guild_id = ?", guildID).First(&guild)

	if guildResult.RowsAffected == 0 {
		guildInfo, err := s.Guild(guildID)
		if err != nil {
			return nil, err
		}
		newGuild := &models.Guild{
			GuildID:                 guildID,
			BetChannelID:            channelId,
			GuildName:               guildInfo.Name,
			PointsPerMessage:        0.5,
			StartingPoints:          1000,
			Pool:                    0,
			CardDrawCost:            10,
			CardDrawCooldownMinutes: 60,
			CardDrawingEnabled:      true,
		}
		newGuildResult := db.Create(newGuild)
		if newGuildResult.Error != nil {
			return nil, newGuildResult.Error
		} else {
			guild = *newGuild
		}
	} else {
		checkGuild, err := s.Guild(guildID)
		if err != nil {
			common.SendError(s, nil, err, db)
		} else {
			if guild.GuildName != checkGuild.Name {
				guild.GuildName = checkGuild.Name
				db.Save(&guild)
			}
		}
	}

	return &guild, nil
}

func SetPointsPerMessage(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
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

	options := i.ApplicationCommandData().Options
	points, err := strconv.ParseFloat(options[0].StringValue(), 64)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	guild, err := GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	guild.PointsPerMessage = points
	db.Save(&guild)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Points per Message set successfully",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
}

func SetStartingPoints(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
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

	options := i.ApplicationCommandData().Options
	points, err := strconv.ParseFloat(options[0].StringValue(), 64)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	guild, err := GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	guild.StartingPoints = points
	db.Save(&guild)

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Starting points set successfully",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
}

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

	guild, err := GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
	} else {
		guild.BetChannelID = i.ChannelID
		db.Save(guild)
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

func SubscribeToTeam(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
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
	}
}

func ToggleCardDrawing(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
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

	guild, err := GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	guild.CardDrawingEnabled = !guild.CardDrawingEnabled
	db.Save(&guild)

	status := "enabled"
	if !guild.CardDrawingEnabled {
		status = "disabled"
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Card drawing has been %s for this server.", status),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
}
