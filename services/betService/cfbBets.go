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
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

var (
	cfbPaginatedOptionsMap = make(map[string][][]discordgo.SelectMenuOption)
	cfbPaginatedOptionsMu  sync.RWMutex
)

// GetCFBPaginatedOptions retrieves paginated options for a given session ID
func GetCFBPaginatedOptions(sessionID string) ([][]discordgo.SelectMenuOption, bool) {
	cfbPaginatedOptionsMu.RLock()
	defer cfbPaginatedOptionsMu.RUnlock()
	options, exists := cfbPaginatedOptionsMap[sessionID]
	return options, exists
}

// CleanupCFBPaginatedOptions removes paginated options for a given session ID
func CleanupCFBPaginatedOptions(sessionID string) {
	cfbPaginatedOptionsMu.Lock()
	defer cfbPaginatedOptionsMu.Unlock()
	delete(cfbPaginatedOptionsMap, sessionID)
}

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
		Title:       "📢 New CFB Bet Created (Will Auto Close & Resolve)",
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

	// Generate unique session ID from interaction ID
	sessionID := i.Interaction.ID

	// Create paginated options
	var paginatedOptions [][]discordgo.SelectMenuOption
	minValues := 1
	for i := 0; i < len(selectOptions); i += 25 {
		end := i + 25
		if end > len(selectOptions) {
			end = len(selectOptions)
		}
		paginatedOptions = append(paginatedOptions, selectOptions[i:end])
	}

	// Store paginated options in thread-safe map
	cfbPaginatedOptionsMu.Lock()
	cfbPaginatedOptionsMap[sessionID] = paginatedOptions
	cfbPaginatedOptionsMu.Unlock()

	currentPage := 0
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Select a game (Page %d/%d):", currentPage+1, len(paginatedOptions)),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("create_cfb_bet_submit_%s", sessionID),
							Placeholder: "Select a game",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     paginatedOptions[currentPage],
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Previous",
							CustomID: fmt.Sprintf("create_cfb_bet_previous_page_%d_%s", currentPage, sessionID),
							Style:    discordgo.PrimaryButton,
							Disabled: true,
						},
						discordgo.Button{
							Label:    "Next",
							CustomID: fmt.Sprintf("create_cfb_bet_next_page_%d_%s", currentPage, sessionID),
							Style:    discordgo.PrimaryButton,
							Disabled: currentPage == len(paginatedOptions)-1,
						},
						discordgo.Button{
							Label:    "Cancel",
							CustomID: fmt.Sprintf("create_cfb_bet_cancel_%s", sessionID),
							Style:    discordgo.DangerButton,
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

func ShowCFBBetTypeSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, betID int) error {
	// Fetch game and line data
	cfbdBet, err := extService.GetCfbdBet(betID)
	if err != nil {
		return err
	}

	line, err := common.PickLine(cfbdBet.Lines)
	if err != nil {
		return err
	}

	homeTeam := cfbdBet.HomeTeam
	awayTeam := cfbdBet.AwayTeam

	// Get spread and odds for ATS
	var spreadValue float64
	homeSpreadOdds := -110
	awaySpreadOdds := -110
	if line.Spread != nil {
		spreadValue = *line.Spread
	}

	// Get moneyline odds
	homeMoneyline := -110
	awayMoneyline := -110
	if line.HomeMoneyline != nil {
		homeMoneyline = *line.HomeMoneyline
	}
	if line.AwayMoneyline != nil {
		awayMoneyline = *line.AwayMoneyline
	}

	// Format spread for display
	spreadDisplay := common.FormatOdds(spreadValue)
	if spreadValue > 0 {
		spreadDisplay = "+" + spreadDisplay
	}

	// Build embed with both bet type options
	description := fmt.Sprintf("**%s @ %s**\n\nSelect the type of bet you want to create:", awayTeam, homeTeam)

	atsField := fmt.Sprintf("**ATS (Against The Spread)**\n1️⃣ %s %s (Odds: %s)\n2️⃣ %s %s (Odds: %s)",
		homeTeam, common.FormatOdds(spreadValue), common.FormatOdds(float64(homeSpreadOdds)),
		awayTeam, common.FormatOdds(spreadValue*-1), common.FormatOdds(float64(awaySpreadOdds)))

	moneylineField := fmt.Sprintf("**Moneyline**\n1️⃣ %s (Odds: %s)\n2️⃣ %s (Odds: %s)",
		homeTeam, common.FormatOdds(float64(homeMoneyline)),
		awayTeam, common.FormatOdds(float64(awayMoneyline)))

	embed := &discordgo.MessageEmbed{
		Title:       "Select Bet Type",
		Description: description,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "📊 ATS Bet",
				Value: atsField,
			},
			{
				Name:  "💰 Moneyline Bet",
				Value: moneylineField,
			},
		},
		Color: 0x3498db,
	}

	// Create buttons for bet type selection
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Create ATS Bet",
							CustomID: fmt.Sprintf("cfb_bet_type_ats_%d", betID),
							Style:    discordgo.PrimaryButton,
						},
						discordgo.Button{
							Label:    "Create Moneyline Bet",
							CustomID: fmt.Sprintf("cfb_bet_type_ml_%d", betID),
							Style:    discordgo.SuccessButton,
						},
						discordgo.Button{
							Label:    "Cancel",
							CustomID: fmt.Sprintf("cfb_bet_type_cancel_%d", betID),
							Style:    discordgo.DangerButton,
						},
					},
				},
			},
		},
	})

	return err
}

func CreateCFBBetFromGameID(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, betID int, betType string) error {
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}
	if !guild.PremiumEnabled {
		return fmt.Errorf("Your server must have the premium subscription in order to enable this feature")
	}

	var dbBet models.Bet
	// Check for existing bet of the same type (ATS has spread, Moneyline doesn't)
	var result *gorm.DB
	if betType == "moneyline" || betType == "ml" {
		// Looking for Moneyline bet (spread IS NULL)
		result = db.Where("cfbd_id = ? AND guild_id = ? AND spread IS NULL", betID, i.GuildID).Find(&dbBet)
	} else {
		// Looking for ATS bet (spread IS NOT NULL)
		result = db.Where("cfbd_id = ? AND guild_id = ? AND spread IS NOT NULL", betID, i.GuildID).Find(&dbBet)
	}
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

		var option1, option2 string
		var odds1, odds2 int
		var spreadValue *float64

		if betType == "moneyline" || betType == "ml" {
			// Moneyline bet
			option1 = cfbdBet.HomeTeam
			option2 = cfbdBet.AwayTeam
			if line.HomeMoneyline != nil {
				odds1 = *line.HomeMoneyline
			} else {
				odds1 = -110
			}
			if line.AwayMoneyline != nil {
				odds2 = *line.AwayMoneyline
			} else {
				odds2 = -110
			}
			spreadValue = nil
		} else {
			// ATS bet (default)
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

			option1 = fmt.Sprintf("%s %s", cfbdBet.HomeTeam, common.FormatOdds(lineValue))
			option2 = fmt.Sprintf("%s %s", cfbdBet.AwayTeam, common.FormatOdds(lineValue*-1))
			odds1 = -110
			odds2 = -110
			spreadValue = &lineValue
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
			Option1:       option1,
			Option2:       option2,
			Odds1:         odds1,
			Odds2:         odds2,
			Active:        true,
			GuildID:       guildID,
			ChannelID:     i.ChannelID,
			GameStartDate: &cfbdBet.StartDate,
			CfbdID:        &cfbdBetID,
			AdminCreated:  common.IsAdmin(s, i),
			Spread:        spreadValue,
		}
		db.Create(&dbBet)
	}

	buttons := messageService.GetBetOnlyButtonsList(dbBet.Option1, dbBet.Option2, dbBet.ID)

	betTypeLabel := "ATS"
	if dbBet.Spread == nil {
		betTypeLabel = "Moneyline"
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📢 New CFB %s Bet Created (Will Auto Close & Resolve)", betTypeLabel),
		Description: dbBet.Description,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  fmt.Sprintf("1️⃣ %s", dbBet.Option1),
				Value: fmt.Sprintf("Odds: %s", common.FormatOdds(float64(dbBet.Odds1))),
			},
			{
				Name:  fmt.Sprintf("2️⃣ %s", dbBet.Option2),
				Value: fmt.Sprintf("Odds: %s", common.FormatOdds(float64(dbBet.Odds2))),
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

// AutoCreateCFBBet automatically creates an ATS bet for a subscribed team game.
// This function ONLY creates ATS bets - Moneyline bets must be created manually via slash command.
// It checks for existing ATS bets only (spread IS NOT NULL) to avoid conflicts with manually created Moneyline bets.
func AutoCreateCFBBet(s *discordgo.Session, db *gorm.DB, guildId string, channelId, gameId string) error {
	guild, err := guildService.GetGuildInfo(s, db, guildId, channelId)
	if err != nil {
		return err
	}
	if !guild.PremiumEnabled {
		return fmt.Errorf("Your server must have the premium subscription in order to enable this feature")
	}

	var dbBet models.Bet
	// AutoCreate only creates ATS bets, so check for existing ATS bet (spread IS NOT NULL)
	// This allows Moneyline bets to be created manually without conflict
	result := db.
		Where("cfbd_id = ? AND paid = 0 AND guild_id = ? AND spread IS NOT NULL", gameId, guildId).
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

		// CFBD API doesn't provide spread odds, so default to -110
		// (Spread odds are typically -110 for ATS bets in CFB)
		odds1 := -110
		odds2 := -110

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
			Odds1:         odds1,
			Odds2:         odds2,
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
			Title:       "📢 New CFB Bet Created (Will Auto Close & Resolve)",
			Description: dbBet.Description,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:  fmt.Sprintf("1️⃣ %s", dbBet.Option1),
					Value: fmt.Sprintf("Odds: %s", common.FormatOdds(float64(dbBet.Odds1))),
				},
				{
					Name:  fmt.Sprintf("2️⃣ %s", dbBet.Option2),
					Value: fmt.Sprintf("Odds: %s", common.FormatOdds(float64(dbBet.Odds2))),
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
