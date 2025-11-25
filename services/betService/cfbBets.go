package betService

import (
	"fmt"
	"math"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/extService"
	"perfectOddsBot/services/guildService"
	"perfectOddsBot/services/messageService"
	"strconv"
	"time"
	_ "time/tzdata"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

var CFBPaginatedOptions [][]discordgo.SelectMenuOption

func CreateCFBBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	options := i.ApplicationCommandData().Options
	betID, err := strconv.Atoi(options[0].StringValue())
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	if !guild.PremiumEnabled {
		common.SendError(s, i, fmt.Errorf("Your server must have the premium subscription in order to enable this feature"), db)
		return
	}

	var dbBet models.Bet
	result := db.
		Where("cfbd_id = ? AND guild_id = ?", betID, i.GuildID).
		Find(&dbBet)
	if result.Error != nil {
		common.SendError(s, i, result.Error, db)
		return
	}

	if result.RowsAffected == 0 {
		cfbdBet, err := extService.GetCfbdBet(betID)
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

		var lineValue float64
		if line.Spread != nil {
			lineValue = *line.Spread
		} else {
			common.SendError(s, nil, err, db)
			return
		}

		// line must be on a 0.5 value to avoid pushes
		if lineValue == math.Trunc(lineValue) {
			lineValue += 0.5
		}

		// Convert to Eastern Time
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}
		t := cfbdBet.StartDate.In(loc)
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
	}

	buttons := messageService.GetBetOnlyButtonsList(dbBet.Option1, dbBet.Option2, dbBet.ID)
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprint("📢 New CFB Bet Created (Will Auto Close & Resolve)"),
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

func CreateCFBBetSelector(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	guild, err := guildService.GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	if !guild.PremiumEnabled {
		common.SendError(s, i, fmt.Errorf("Your server must have the premium subscription in order to enable this feature"), db)
		return
	}

	bettingLines, err := extService.GetCFBGames()
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	conferenceList := []string{"Big Ten", "ACC", "SEC", "Big 12", "Pac-12"}
	var selectOptions []discordgo.SelectMenuOption
	for _, bet := range bettingLines {
		// Only include games from specific conferences that haven't finished and have lines
		if (common.Contains(conferenceList, bet.HomeConference) || common.Contains(conferenceList, bet.AwayConference)) &&
			bet.HomeScore == nil && bet.AwayScore == nil {
			line, lineErr := common.PickLine(bet.Lines)
			if lineErr != nil {
				continue
			}

			label := fmt.Sprintf("%s @ %s", bet.AwayTeam, bet.HomeTeam)
			// Discord select menu labels have a max length of 100 characters
			if len(label) > 100 {
				label = label[:97] + "..."
			}

			description := line.FormattedSpread
			if len(description) > 100 {
				description = description[:97] + "..."
			}

			selectOptions = append(selectOptions, discordgo.SelectMenuOption{
				Label:       label,
				Value:       strconv.Itoa(bet.ID),
				Description: description,
				Emoji:       nil,
				Default:     false,
			})
		}
	}

	if len(selectOptions) == 0 {
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "There are no games available to create bets for.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	// Reset paginated options
	CFBPaginatedOptions = [][]discordgo.SelectMenuOption{}
	minValues := 1
	for i := 0; i < len(selectOptions); i += 25 {
		end := i + 25
		if end > len(selectOptions) {
			end = len(selectOptions)
		}
		CFBPaginatedOptions = append(CFBPaginatedOptions, selectOptions[i:end])
	}

	currentPage := 0
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Select a game (Page %d/%d):", currentPage+1, len(CFBPaginatedOptions)),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    "create_cfb_bet_submit",
							Placeholder: "Select a game",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     CFBPaginatedOptions[currentPage],
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Previous",
							CustomID: "create_cfb_bet_previous_page_0",
							Style:    discordgo.PrimaryButton,
							Disabled: true,
						},
						discordgo.Button{
							Label:    "Next",
							CustomID: "create_cfb_bet_next_page_0",
							Style:    discordgo.PrimaryButton,
							Disabled: currentPage == len(CFBPaginatedOptions)-1,
						},
					},
				},
			},
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	return
}

func CreateCFBBetFromGameID(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, betID int) error {
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}
	if !guild.PremiumEnabled {
		return fmt.Errorf("Your server must have the premium subscription in order to enable this feature")
	}

	var dbBet models.Bet
	result := db.
		Where("cfbd_id = ? AND guild_id = ?", betID, i.GuildID).
		Find(&dbBet)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		cfbdBet, err := extService.GetCfbdBet(betID)
		if err != nil {
			return err
		}

		line, err := common.PickLine(cfbdBet.Lines)
		if err != nil {
			return err
		}

		cfbdBetID := strconv.Itoa(cfbdBet.ID)

		var lineValue float64
		if line.Spread != nil {
			lineValue = *line.Spread
		} else {
			return fmt.Errorf("no spread available")
		}

		// line must be on a 0.5 value to avoid pushes
		if lineValue == math.Trunc(lineValue) {
			lineValue += 0.5
		}

		// Convert to Eastern Time
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			return err
		}
		t := cfbdBet.StartDate.In(loc)
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
	}

	buttons := messageService.GetBetOnlyButtonsList(dbBet.Option1, dbBet.Option2, dbBet.ID)
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprint("📢 New CFB Bet Created (Will Auto Close & Resolve)"),
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
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &interactionData,
	})
	if err != nil {
		return err
	}

	msg, err := s.InteractionResponse(i.Interaction)
	if err != nil {
		return err
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

	return nil
}

func AutoCreateCFBBet(s *discordgo.Session, db *gorm.DB, guildId string, channelId, gameId string) error {
	guild, err := guildService.GetGuildInfo(s, db, guildId, channelId)
	if err != nil {
		return err
	}
	if !guild.PremiumEnabled {
		return fmt.Errorf("Your server must have the premium subscription in order to enable this feature")
	}

	var dbBet models.Bet
	result := db.
		Where("cfbd_id = ? AND paid = 0 AND guild_id = ?", gameId, guildId).
		Find(&dbBet)
	if result.Error != nil {
		return result.Error
	}

	gameInt, _ := strconv.Atoi(gameId)
	if result.RowsAffected == 0 {
		cfbdBet, err := extService.GetCfbdBet(gameInt)
		if err != nil {
			return err
		}

		line, err := common.PickLine(cfbdBet.Lines)
		if err != nil {
			return err
		}

		cfbdBetID := strconv.Itoa(cfbdBet.ID)

		var lineValue float64
		if line.Spread != nil {
			lineValue = *line.Spread
		} else {
			common.SendError(s, nil, err, db)
			return err
		}

		// line must be on a 0.5 value to avoid pushes
		if lineValue == math.Trunc(lineValue) {
			lineValue += 0.5
		}

		// Convert to Eastern Time
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			return err
		}
		t := cfbdBet.StartDate.In(loc)
		formattedTime := t.Format("Mon 03:04 pm MST")

		dbBet = models.Bet{
			Description:   fmt.Sprintf("%s @ %s (%s)", cfbdBet.AwayTeam, cfbdBet.HomeTeam, formattedTime),
			Option1:       fmt.Sprintf("%s %s", cfbdBet.HomeTeam, common.FormatOdds(lineValue)),
			Option2:       fmt.Sprintf("%s %s", cfbdBet.AwayTeam, common.FormatOdds(lineValue*-1)),
			Odds1:         -110,
			Odds2:         -110,
			Active:        true,
			GuildID:       guildId,
			ChannelID:     channelId,
			GameStartDate: &cfbdBet.StartDate,
			CfbdID:        &cfbdBetID,
			AdminCreated:  true,
			Spread:        &lineValue,
		}
		db.Create(&dbBet)

		buttons := messageService.GetBetOnlyButtonsList(dbBet.Option1, dbBet.Option2, dbBet.ID)
		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprint("📢 New CFB Bet Created (Will Auto Close & Resolve)"),
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

		msg, err := s.ChannelMessageSendComplex(guild.BetChannelID, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: buttons,
				},
			},
		})
		if err != nil {
			return err
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

		db.Save(&dbBet)
	}

	return nil
}
