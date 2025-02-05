package betService

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"math"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/extService"
	"perfectOddsBot/services/guildService"
	"perfectOddsBot/services/messageService"
	"strconv"
	"time"
	_ "time/tzdata"
)

func CreateCBBBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
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
		Where("espn_id = ? AND paid = 0 AND guild_id = ?", betID, i.GuildID).
		Find(&dbBet)
	if result.Error != nil {
		common.SendError(s, i, result.Error, db)
		return
	}

	if result.RowsAffected == 0 {
		linesList, err := extService.GetCbbLines(betID)
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}

		line, err := common.PickESPNLine(linesList)
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}

		espnID := strconv.Itoa(betID)
		cbbEvent, err := extService.GetCbbGame(espnID)
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}
		homeTeam := ""
		awayTeam := ""
		for _, competitor := range cbbEvent.Competitions[0].Competitors {
			if competitor.HomeAway == "home" {
				homeTeam = competitor.Team.ShortDisplayName
			}
			if competitor.HomeAway == "away" {
				awayTeam = competitor.Team.ShortDisplayName
			}
		}

		fmt.Println(cbbEvent.Date)
		espnDateLayout := "2006-01-02T15:04Z"
		utcTime, err := time.Parse(espnDateLayout, cbbEvent.Date)
		if err != nil {
			common.SendError(s, i, errors.New(fmt.Sprintf("err parsing time: %v", err)), db)
			return
		}

		// Convert to Eastern Time
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			common.SendError(s, i, errors.New(fmt.Sprintf("err converting time: %v", err)), db)
			return
		}
		t := utcTime.In(loc)
		formattedTime := t.Format("Mon 03:04 pm MST")

		// line must be on a 0.5 value to avoid pushes
		lineValue := line.Spread
		if lineValue == math.Trunc(lineValue) {
			lineValue += 0.5
		}

		dbBet = models.Bet{
			Description:   fmt.Sprintf("%s @ %s (%s)", awayTeam, homeTeam, formattedTime),
			Option1:       fmt.Sprintf("%s %s", homeTeam, common.FormatOdds(line.Spread)),
			Option2:       fmt.Sprintf("%s %s", awayTeam, common.FormatOdds(line.Spread*-1)),
			Odds1:         -110,
			Odds2:         -110,
			Active:        true,
			GuildID:       guildID,
			ChannelID:     i.ChannelID,
			GameStartDate: &utcTime,
			EspnID:        &espnID,
			AdminCreated:  common.IsAdmin(s, i),
			Spread:        &line.Spread,
		}
		db.Create(&dbBet)
	}

	buttons := messageService.GetBetOnlyButtonsList(dbBet.Option1, dbBet.Option2, dbBet.ID)
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprint("📢 New CBB Bet Created (Will Auto Close & Resolve)"),
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

func AutoCreateCBBBet(s *discordgo.Session, db *gorm.DB, guildId string, channelId, gameId string) error {
	guild, err := guildService.GetGuildInfo(s, db, guildId, channelId)
	if err != nil {
		return err
	}
	if !guild.PremiumEnabled {
		return fmt.Errorf("Your server must have the premium subscription in order to enable this feature")
	}

	var dbBet models.Bet
	result := db.
		Where("espn_id = ? AND guild_id = ?", gameId, guildId).
		Find(&dbBet)
	if result.Error != nil {
		return result.Error
	}

	gameInt, _ := strconv.Atoi(gameId)
	if result.RowsAffected == 0 {
		linesList, err := extService.GetCbbLines(gameInt)
		if err != nil {
			return err
		}

		line, err := common.PickESPNLine(linesList)
		if err != nil {
			return err
		}

		cbbEvent, err := extService.GetCbbGame(gameId)
		if err != nil {
			return err
		}
		homeTeam := ""
		awayTeam := ""
		for _, competitor := range cbbEvent.Competitions[0].Competitors {
			if competitor.HomeAway == "home" {
				homeTeam = competitor.Team.ShortDisplayName
			}
			if competitor.HomeAway == "away" {
				awayTeam = competitor.Team.ShortDisplayName
			}
		}

		fmt.Println(cbbEvent.Date)
		espnDateLayout := "2006-01-02T15:04Z"
		utcTime, err := time.Parse(espnDateLayout, cbbEvent.Date)
		if err != nil {
			return errors.New(fmt.Sprintf("err parsing time: %v", err))
		}

		// Convert to Eastern Time
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			return errors.New(fmt.Sprintf("err converting time: %v", err))
		}
		t := utcTime.In(loc)
		formattedTime := t.Format("Mon 03:04 pm MST")

		// line must be on a 0.5 value to avoid pushes
		lineValue := line.Spread
		if lineValue == math.Trunc(lineValue) {
			lineValue += 0.5
		}

		dbBet = models.Bet{
			Description:   fmt.Sprintf("%s @ %s (%s)", awayTeam, homeTeam, formattedTime),
			Option1:       fmt.Sprintf("%s %s", homeTeam, common.FormatOdds(line.Spread)),
			Option2:       fmt.Sprintf("%s %s", awayTeam, common.FormatOdds(line.Spread*-1)),
			Odds1:         -110,
			Odds2:         -110,
			Active:        true,
			GuildID:       guildId,
			ChannelID:     channelId,
			GameStartDate: &utcTime,
			EspnID:        &gameId,
			AdminCreated:  true,
			Spread:        &line.Spread,
		}
		db.Create(&dbBet)

		buttons := messageService.GetBetOnlyButtonsList(dbBet.Option1, dbBet.Option2, dbBet.ID)
		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprint("📢 New CBB Bet Created (Will Auto Close & Resolve)"),
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
