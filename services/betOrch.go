package services

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
)

func CreateBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	if !IsAdmin(s, i) {
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

	options := i.ApplicationCommandData().Options
	description := options[0].StringValue()
	option1 := options[1].StringValue()
	option2 := options[2].StringValue()
	odds1 := int(options[3].IntValue())
	odds2 := int(options[4].IntValue())
	guildID := i.GuildID

	bet := models.Bet{
		Description: description,
		Option1:     option1,
		Option2:     option2,
		Odds1:       odds1,
		Odds2:       odds2,
		Active:      true,
		GuildID:     guildID,
	}
	db.Create(&bet)

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprint("📢 New Bet Created"),
		Description: description,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  fmt.Sprintf("1️⃣ %s", option1),
				Value: fmt.Sprintf("Odds: %s", FormatOdds(odds1)),
			},
			{
				Name:  fmt.Sprintf("2️⃣ %s", option2),
				Value: fmt.Sprintf("Odds: %s", FormatOdds(odds2)),
			},
		},
		Color: 0x3498db,
	}

	buttons := []discordgo.MessageComponent{
		discordgo.Button{
			Label:    option1,
			Style:    discordgo.PrimaryButton,
			CustomID: fmt.Sprintf("bet_%d_option1", bet.ID),
			Emoji: &discordgo.ComponentEmoji{
				Name: "🟡",
			},
		},
		discordgo.Button{
			Label:    option2,
			Style:    discordgo.SuccessButton,
			CustomID: fmt.Sprintf("bet_%d_option2", bet.ID),
			Emoji: &discordgo.ComponentEmoji{
				Name: "🟡",
			},
		},
	}

	if IsAdmin(s, i) {
		buttons = append(buttons,
			discordgo.Button{
				Label:    "Close Betting",
				Style:    discordgo.DangerButton,
				CustomID: fmt.Sprintf("lock_bet_%d", bet.ID),
				Emoji: &discordgo.ComponentEmoji{
					Name: "🔒",
				},
			},
			discordgo.Button{
				Label:    "Resolve Bet",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("resolve_bet_%d", bet.ID),
				Emoji: &discordgo.ComponentEmoji{
					Name: "✅",
				},
			},
		)
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: buttons,
				},
			},
		},
	})
	if err != nil {
		return
	}
}

func ResolveBetByID(s *discordgo.Session, i *discordgo.InteractionCreate, betID int, winningOption int, db *gorm.DB) {
	var bet models.Bet
	winnersList := ""
	result := db.First(&bet, "id = ? AND guild_id = ?", betID, i.GuildID)
	if result.Error != nil || bet.ID == 0 {
		response := "Bet not found or already resolved."
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			fmt.Println(err)
			return
		}
		return
	}

	var entries []models.BetEntry
	db.Where("bet_id = ? AND `option` = ?", bet.ID, winningOption).Find(&entries)

	totalPayout := 0
	for _, entry := range entries {
		var user models.User
		db.First(&user, "id = ?", entry.UserID)
		if user.ID == 0 {
			continue
		}

		payout := CalculatePayout(entry.Amount, winningOption, bet)
		user.Points += payout
		db.Save(&user)
		totalPayout += payout

		if payout > 0 {
			username := GetUsername(s, i, user.DiscordID)
			winnersList += fmt.Sprintf("%s - $%d\n", username, payout)
		}
	}

	bet.Active = false
	db.Model(&bet).UpdateColumn("paid", true).UpdateColumn("active", false)

	winningOptionName := bet.Option1
	if winningOption == 2 {
		winningOptionName = bet.Option2
	}

	response := fmt.Sprintf("Bet '%s' has been resolved!\n**%s** is the winning option.\nTotal payout: **%d** points.\n**Winners:**\n%s", bet.Description, winningOptionName, totalPayout, winnersList)
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

func ResolveBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	options := i.ApplicationCommandData().Options
	betid := int(options[0].IntValue())
	option := int(options[1].IntValue())
	ResolveBetByID(s, i, betid, option, db)

}

func MyOpenBets(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	var bets []models.BetEntry
	userID := i.Member.User.ID

	result := db.
		Preload("Bet").
		Joins("JOIN bets ON bet_entries.bet_id = bets.id").
		Joins("JOIN users ON bet_entries.user_id = users.id").
		Where("users.discord_id = ? AND bets.paid = 0 AND bets.guild_id = ?", userID, i.GuildID).
		Find(&bets)
	if result.Error != nil {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error finding active bets.",
				Flags:   discordgo.MessageFlagsEphemeral, // Send this response as ephemeral
			},
		})
		if err != nil {
			fmt.Println(err)
			return
		}
		return
	}

	var response string
	if len(bets) == 0 {
		response = "You have no active bets."
	} else {
		response = fmt.Sprintf("You have %d active bets:\n", len(bets))
		for _, bet := range bets {
			if bet.Option == 1 {
				response += fmt.Sprintf("* `%s` - $%d on %s.\n", bet.Bet.Description, bet.Amount, bet.Bet.Option1)
			} else {
				response += fmt.Sprintf("* `%s` - $%d on %s.\n", bet.Bet.Description, bet.Amount, bet.Bet.Option2)
			}
		}
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
}
