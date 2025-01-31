package betService

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/messageService"
	"strconv"
	"time"
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
	if options[3] != nil {
		odds1 = int(options[3].IntValue())
	}
	if options[4] != nil {
		odds2 = int(options[4].IntValue())
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
	loserList := ""
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

	var entries []models.BetEntry
	db.Where("bet_id = ? AND `option` = ?", bet.ID, winningOption).Find(&entries)

	totalPayout := 0
	for _, entry := range entries {
		var user models.User
		db.First(&user, "id = ?", entry.UserID)
		if user.ID == 0 {
			continue
		}

		payout := common.CalculatePayout(entry.Amount, winningOption, bet)
		user.Points += payout
		db.Save(&user)
		totalPayout += payout

		if payout > 0 {
			username := common.GetUsername(s, user.GuildID, user.DiscordID)
			winnersList += fmt.Sprintf("%s - Won $%d\n", username, payout)
		}
	}

	bet.Active = false
	db.Model(&bet).UpdateColumn("paid", true).UpdateColumn("active", false)

	winningOptionName := bet.Option1
	if winningOption == 2 {
		winningOptionName = bet.Option2
	}

	response := fmt.Sprintf("Bet '%s' has been resolved!\n**%s** is the winning option.\nTotal payout: **%d** points.\n**Winners:**\n%s\n**Losers:**\n%s", bet.Description, winningOptionName, totalPayout, winnersList, loserList)
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
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
					response += fmt.Sprintf("* `%s` - $%d on %.1f.\n", bet.Bet.Description, bet.Amount, bet.Bet.Option1)
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

func CreateCFBBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	options := i.ApplicationCommandData().Options
	betID, err := strconv.Atoi(options[0].StringValue())
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	guildID := i.GuildID

	var dbBet models.Bet
	result := db.
		Where("cfbd_id = ? AND paid = 0 AND guild_id = ?", betID, i.GuildID).
		Find(&dbBet)
	if result.Error != nil {
		common.SendError(s, i, result.Error, db)
		return
	}

	// Convert to Eastern Time
	est, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}

	if result.RowsAffected == 0 {
		cfbdBet, err := common.GetCfbdBet(betID)
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}

		line, err := common.PickLine(cfbdBet.Lines)
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}

		cfbdBetID := strconv.Itoa(cfbdBet.ID)

		lineValue, err := strconv.ParseFloat(line.Spread, 64)
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}

		// game start time
		t := cfbdBet.StartDate.In(est)
		// Get the current time in EST
		currentTimeEST := time.Now().In(est)

		if currentTimeEST.After(t) {
			cantBetMsg := "Game has already begun."
			if cfbdBet.HomeScore != nil && cfbdBet.AwayScore != nil {
				cantBetMsg = "Game has already ended."
			}
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("%s You can no longer create a bet for this game.", cantBetMsg),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				common.SendError(s, i, err, db)
			}
			return
		}

		formattedTime := t.Format("Mon 03:04 pm MST")
		dbBet = models.Bet{
			Description:   fmt.Sprintf("%s @ %s (%s)", cfbdBet.AwayTeam, cfbdBet.HomeTeam, formattedTime),
			Option1:       fmt.Sprintf("%s %s", cfbdBet.HomeTeam, common.FormatOdds(lineValue)),
			Option2:       fmt.Sprintf("%s %s", cfbdBet.AwayTeam, common.FormatOdds(lineValue*-1)),
			Odds1:         -110,
			Odds2:         -110,
			Active:        true,
			GuildID:       guildID,
			ChannelID:     i.ChannelID,
			GameStartDate: &cfbdBet.StartDate,
			CfbdID:        &cfbdBetID,
			AdminCreated:  common.IsAdmin(s, i),
			Spread:        &lineValue,
		}
		db.Create(&dbBet)
	} else {
		// game start time
		t := dbBet.GameStartDate.In(est)
		// Get the current time in EST
		currentTimeEST := time.Now().In(est)

		if currentTimeEST.After(t) {
			cantBetMsg := "Game has already begun."
			if dbBet.Paid {
				cantBetMsg = "Game has already ended."
			}
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("%s You can no longer create a bet for this game.", cantBetMsg),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				common.SendError(s, i, err, db)
			}
			return
		}
	}

	buttons := messageService.GetBetOnlyButtonsList(dbBet.Option1, dbBet.Option2, dbBet.ID)
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprint("📢 New Bet Created (Will Auto Close & Resolve)"),
		Description: dbBet.Description,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  fmt.Sprintf("1️⃣ %s", dbBet.Option1),
				Value: fmt.Sprintf("Odds: %s", common.FormatOdds(-110)),
			},
			{
				Name:  fmt.Sprintf("2️⃣ %s", dbBet.Option2),
				Value: fmt.Sprintf("Odds: %s", common.FormatOdds(-110)),
			},
		},
		Color: 0x3498db,
	}

	interactionData := discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: buttons,
			},
		},
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &interactionData,
	})

	msg, err := s.InteractionResponse(i.Interaction)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	if dbBet.MessageID != nil {
		db.Create(&models.BetMessage{
			Active:    true,
			BetID:     dbBet.ID,
			MessageID: &msg.ID,
			ChannelID: msg.ChannelID,
		})
	} else {
		dbBet.MessageID = &msg.ID
	}

	if common.IsAdmin(s, i) {
		dbBet.AdminCreated = common.IsAdmin(s, i)
	}

	db.Save(&dbBet)

	return
}
