package services

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
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
	}
	
	// Update username from interaction member
	username := common.GetUsernameFromUser(i.Member.User)
	common.UpdateUserUsername(db, &user, username)
	
	if result.RowsAffected == 1 {
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

	// Update username from target user
	username := common.GetUsernameFromUser(targetUser)
	common.UpdateUserUsername(db, &user, username)

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

func ShowStats(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
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

	totalBets := user.TotalBetsWon + user.TotalBetsLost
	netPoints := user.TotalPointsWon - user.TotalPointsLost

	var winRate float64
	if totalBets > 0 {
		winRate = float64(user.TotalBetsWon) / float64(totalBets) * 100
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📊 Your Betting Statistics",
		Description: fmt.Sprintf("Statistics for <@%s>", userID),
		Color:       0xe67e22,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Bets Won",
				Value:  fmt.Sprintf("%d", user.TotalBetsWon),
				Inline: true,
			},
			{
				Name:   "Bets Lost",
				Value:  fmt.Sprintf("%d", user.TotalBetsLost),
				Inline: true,
			},
			{
				Name:   "Total Bets",
				Value:  fmt.Sprintf("%d", totalBets),
				Inline: true,
			},
			{
				Name:   "Win Rate",
				Value:  fmt.Sprintf("%.1f%%", winRate),
				Inline: true,
			},
			{
				Name:   "Points Won",
				Value:  fmt.Sprintf("%.1f", user.TotalPointsWon),
				Inline: true,
			},
			{
				Name:   "Points Lost",
				Value:  fmt.Sprintf("%.1f", user.TotalPointsLost),
				Inline: true,
			},
			{
				Name:   "Net Points",
				Value:  fmt.Sprintf("%.1f", netPoints),
				Inline: true,
			},
			{
				Name:   "Current Points",
				Value:  fmt.Sprintf("%.1f", user.Points),
				Inline: true,
			},
		},
	}

	if totalBets == 0 {
		embed.Description = fmt.Sprintf("Statistics for <@%s>\n\nYou haven't placed any bets yet!", userID)
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
		return
	}
}
