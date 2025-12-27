package betService

import (
	"fmt"
	"strings"

	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"perfectOddsBot/services/messageService"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func CreateCustomBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
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
	description := options[0].StringValue()
	option1 := options[1].StringValue()
	option2 := options[2].StringValue()
	odds1 := -110
	odds2 := -110
	if len(options) > 3 {
		if options[3] != nil {
			odds1 = int(options[3].IntValue())
		}
		if len(options) > 4 && options[4] != nil {
			odds2 = int(options[4].IntValue())
		}
	}
	guildID := i.GuildID

	bet := models.Bet{
		Description:  description,
		Option1:      option1,
		Option2:      option2,
		Odds1:        odds1,
		Odds2:        odds2,
		Active:       true,
		GuildID:      guildID,
		ChannelID:    i.ChannelID,
		AdminCreated: true,
	}
	db.Create(&bet)

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprint("📢 New Bet Created"),
		Description: description,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  fmt.Sprintf("1️⃣ %s", option1),
				Value: fmt.Sprintf("Odds: %s", common.FormatOdds(float64(odds1))),
			},
			{
				Name:  fmt.Sprintf("2️⃣ %s", option2),
				Value: fmt.Sprintf("Odds: %s", common.FormatOdds(float64(odds2))),
			},
		},
		Color: 0x3498db,
	}

	buttons := messageService.GetAllButtonList(s, i, option1, option2, bet.ID)

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

	msg, err := s.InteractionResponse(i.Interaction)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	if bet.MessageID == nil {
		bet.MessageID = &msg.ID
		db.Save(&bet)
	}

	return
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
			common.SendError(s, i, err, db)
			return
		}
		return
	}

	// Get guild info for pool accumulation
	guild, err := guildService.GetGuildInfo(s, db, bet.GuildID, bet.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	// Get all entries for this bet
	var entries []models.BetEntry
	db.Where("bet_id = ?", bet.ID).Find(&entries)

	totalPayout := 0.0
	lostPoolAmount := 0.0
	for _, entry := range entries {
		var user models.User
		db.First(&user, "id = ?", entry.UserID)
		if user.ID == 0 {
			continue
		}

		if entry.Option == winningOption {
			// Winning entry - calculate payout
			payout := common.CalculatePayout(entry.Amount, winningOption, bet)
			user.Points += payout
			user.TotalBetsWon++
			user.TotalPointsWon += payout
			db.Save(&user)
			totalPayout += payout

			if payout > 0 {
				username := common.GetUsernameWithDB(db, s, user.GuildID, user.DiscordID)
				winnersList += fmt.Sprintf("%s - Won $%.1f\n", username, payout)
			}
		} else {
			// Losing entry - add to pool
			user.TotalBetsLost++
			user.TotalPointsLost += float64(entry.Amount)
			db.Save(&user)
			lostPoolAmount += float64(entry.Amount)
		}
	}

	// Add lost bet amounts to guild pool (atomic update to prevent race conditions)
	if lostPoolAmount > 0 {
		db.Model(&models.Guild{}).Where("id = ?", guild.ID).UpdateColumn("pool", gorm.Expr("pool + ?", lostPoolAmount))
	}

	bet.Active = false
	db.Model(&bet).UpdateColumn("paid", true).UpdateColumn("active", false)

	winningOptionName := bet.Option1
	if winningOption == 2 {
		winningOptionName = bet.Option2
	}

	winnersText := strings.TrimSpace(winnersList)
	embed := messageService.BuildBetResolutionEmbed(
		bet.Description,
		fmt.Sprintf("Winning option: **%s**", winningOptionName),
		totalPayout,
		winnersText,
		"",
	)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
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
			common.SendError(s, i, err, db)
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
			if bet.Spread != nil {
				if bet.Option == 1 {
					response += fmt.Sprintf("* `%s` - $%d on Home %s.\n", bet.Bet.Description, bet.Amount, common.FormatOdds(*bet.Spread))
				} else {
					spreadVal := *bet.Spread
					spreadVal = *bet.Spread * -1

					response += fmt.Sprintf("* `%s` - $%d on Away %s.\n", bet.Bet.Description, bet.Amount, common.FormatOdds(spreadVal))
				}
			} else {
				if bet.Option == 1 {
					response += fmt.Sprintf("* `%s` - $%d on %s.\n", bet.Bet.Description, bet.Amount, bet.Bet.Option1)
				} else {
					response += fmt.Sprintf("* `%s` - $%d on %s.\n", bet.Bet.Description, bet.Amount, bet.Bet.Option2)
				}
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
		common.SendError(s, i, err, db)
		return
	}
}
