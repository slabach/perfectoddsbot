package betService

import (
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
