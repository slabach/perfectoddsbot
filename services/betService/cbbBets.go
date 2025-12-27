package betService

import (
	"errors"
	"fmt"
	"math"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/extService"
	"perfectOddsBot/services/guildService"
	"perfectOddsBot/services/messageService"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

var (
	cbbPaginatedOptionsMap = make(map[string][][]discordgo.SelectMenuOption)
	cbbPaginatedOptionsMu  sync.RWMutex
)

// GetCBBPaginatedOptions retrieves paginated options for a given session ID
func GetCBBPaginatedOptions(sessionID string) ([][]discordgo.SelectMenuOption, bool) {
	cbbPaginatedOptionsMu.RLock()
	defer cbbPaginatedOptionsMu.RUnlock()
	options, exists := cbbPaginatedOptionsMap[sessionID]
	return options, exists
}

// CleanupCBBPaginatedOptions removes paginated options for a given session ID
func CleanupCBBPaginatedOptions(sessionID string) {
	cbbPaginatedOptionsMu.Lock()
	defer cbbPaginatedOptionsMu.Unlock()
	delete(cbbPaginatedOptionsMap, sessionID)
}

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
			isHome := false
			if line.HomeTeamOdds.Team.Ref != "" {
				if strings.Contains(line.HomeTeamOdds.Team.Ref, fmt.Sprintf("/teams/%s?", competitor.ID)) {
					isHome = true
				}
			} else if competitor.HomeAway == "home" {
				isHome = true
			}

			if isHome {
				homeTeam = competitor.Team.ShortDisplayName
			} else {
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
			Description:   fmt.Sprintf("%s @ %s (%s)\n- Broadcast: %s", awayTeam, homeTeam, formattedTime, cbbEvent.Competitions[0].Broadcast),
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
		Title:       "📢 New CBB Bet Created (Will Auto Close & Resolve)",
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

func CreateCBBBetSelector(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	guild, err := guildService.GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	if !guild.PremiumEnabled {
		common.SendError(s, i, fmt.Errorf("Your server must have the premium subscription in order to enable this feature"), db)
		return
	}

	events, err := extService.GetCbbGames()
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	var selectOptions []discordgo.SelectMenuOption
	for _, event := range events {
		// Only include games that are not final
		if len(event.Competitions) > 0 && event.Competitions[0].Status.Type.Name != "STATUS_FINAL" {
			// Get team names for display
			homeTeam := ""
			awayTeam := ""
			for _, competitor := range event.Competitions[0].Competitors {
				if competitor.HomeAway == "home" {
					homeTeam = competitor.Team.ShortDisplayName
				}
				if competitor.HomeAway == "away" {
					awayTeam = competitor.Team.ShortDisplayName
				}
			}

			// Fetch lines for this game
			eventID, err := strconv.Atoi(event.ID)
			if err != nil {
				continue
			}

			linesList, err := extService.GetCbbLines(eventID)
			if err != nil {
				continue // Skip games without lines
			}

			line, lineErr := common.PickESPNLine(linesList)
			if lineErr != nil {
				continue // Skip games without valid lines
			}

			label := fmt.Sprintf("%s @ %s", awayTeam, homeTeam)
			// Discord select menu labels have a max length of 100 characters
			if len(label) > 100 {
				label = label[:97] + "..."
			}

			description := line.Details
			if len(description) > 100 {
				description = description[:97] + "..."
			}

			selectOptions = append(selectOptions, discordgo.SelectMenuOption{
				Label:       label,
				Value:       event.ID,
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
	cbbPaginatedOptionsMu.Lock()
	cbbPaginatedOptionsMap[sessionID] = paginatedOptions
	cbbPaginatedOptionsMu.Unlock()

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
							CustomID:    fmt.Sprintf("create_cbb_bet_submit_%s", sessionID),
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
							CustomID: fmt.Sprintf("create_cbb_bet_previous_page_%d_%s", currentPage, sessionID),
							Style:    discordgo.PrimaryButton,
							Disabled: true,
						},
						discordgo.Button{
							Label:    "Next",
							CustomID: fmt.Sprintf("create_cbb_bet_next_page_%d_%s", currentPage, sessionID),
							Style:    discordgo.PrimaryButton,
							Disabled: currentPage == len(paginatedOptions)-1,
						},
						discordgo.Button{
							Label:    "Cancel",
							CustomID: fmt.Sprintf("create_cbb_bet_cancel_%s", sessionID),
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

func ShowCBBBetTypeSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, betID int) error {
	// Fetch game and line data
	linesList, err := extService.GetCbbLines(betID)
	if err != nil {
		return err
	}

	line, err := common.PickESPNLine(linesList)
	if err != nil {
		return err
	}

	espnID := strconv.Itoa(betID)
	cbbEvent, err := extService.GetCbbGame(espnID)
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

	// Get spread and odds for ATS
	spreadValue := line.Spread
	homeSpreadOdds := -110
	awaySpreadOdds := -110
	if line.HomeTeamOdds.SpreadOdds != 0 {
		homeSpreadOdds = int(line.HomeTeamOdds.SpreadOdds)
	}
	if line.AwayTeamOdds.SpreadOdds != 0 {
		awaySpreadOdds = int(line.AwayTeamOdds.SpreadOdds)
	}

	// Check if moneyline odds are available (both must be non-zero)
	moneylineAvailable := line.HomeTeamOdds.MoneyLine != 0 && line.AwayTeamOdds.MoneyLine != 0

	// Get moneyline odds (only if available)
	var homeMoneyline, awayMoneyline int
	if moneylineAvailable {
		homeMoneyline = line.HomeTeamOdds.MoneyLine
		awayMoneyline = line.AwayTeamOdds.MoneyLine
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

	// Build embed fields
	embedFields := []*discordgo.MessageEmbedField{
		{
			Name:  "📊 ATS Bet",
			Value: atsField,
		},
	}

	// Only add moneyline field and button if odds are available
	var buttons []discordgo.MessageComponent
	if moneylineAvailable {
		moneylineField := fmt.Sprintf("**Moneyline**\n1️⃣ %s (Odds: %s)\n2️⃣ %s (Odds: %s)",
			homeTeam, common.FormatOdds(float64(homeMoneyline)),
			awayTeam, common.FormatOdds(float64(awayMoneyline)))
		embedFields = append(embedFields, &discordgo.MessageEmbedField{
			Name:  "💰 Moneyline Bet",
			Value: moneylineField,
		})
		buttons = []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Create ATS Bet",
				CustomID: fmt.Sprintf("cbb_bet_type_ats_%d", betID),
				Style:    discordgo.PrimaryButton,
			},
			discordgo.Button{
				Label:    "Create Moneyline Bet",
				CustomID: fmt.Sprintf("cbb_bet_type_ml_%d", betID),
				Style:    discordgo.SuccessButton,
			},
			discordgo.Button{
				Label:    "Cancel",
				CustomID: fmt.Sprintf("cbb_bet_type_cancel_%d", betID),
				Style:    discordgo.DangerButton,
			},
		}
	} else {
		embedFields = append(embedFields, &discordgo.MessageEmbedField{
			Name:  "💰 Moneyline Bet",
			Value: "No moneyline bet available",
		})
		buttons = []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Create ATS Bet",
				CustomID: fmt.Sprintf("cbb_bet_type_ats_%d", betID),
				Style:    discordgo.PrimaryButton,
			},
			discordgo.Button{
				Label:    "Cancel",
				CustomID: fmt.Sprintf("cbb_bet_type_cancel_%d", betID),
				Style:    discordgo.DangerButton,
			},
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Select Bet Type",
		Description: description,
		Fields:      embedFields,
		Color:       0x3498db,
	}

	// Create buttons for bet type selection
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: buttons,
				},
			},
		},
	})

	return err
}

func CreateCBBBetFromGameID(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, betID int, betType string) error {
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
		result = db.Where("espn_id = ? AND paid = 0 AND guild_id = ? AND spread IS NULL", betID, i.GuildID).Find(&dbBet)
	} else {
		// Looking for ATS bet (spread IS NOT NULL)
		result = db.Where("espn_id = ? AND paid = 0 AND guild_id = ? AND spread IS NOT NULL", betID, i.GuildID).Find(&dbBet)
	}
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		linesList, err := extService.GetCbbLines(betID)
		if err != nil {
			return err
		}

		line, err := common.PickESPNLine(linesList)
		if err != nil {
			return err
		}

		espnID := strconv.Itoa(betID)
		cbbEvent, err := extService.GetCbbGame(espnID)
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
			return fmt.Errorf("err parsing time: %v", err)
		}

		// Convert to Eastern Time
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			return fmt.Errorf("err converting time: %v", err)
		}
		t := utcTime.In(loc)
		formattedTime := t.Format("Mon 03:04 pm MST")

		var option1, option2 string
		var odds1, odds2 int
		var spreadValue *float64

		if betType == "moneyline" || betType == "ml" {
			// Moneyline bet - validate odds are available
			if line.HomeTeamOdds.MoneyLine == 0 || line.AwayTeamOdds.MoneyLine == 0 {
				return fmt.Errorf("moneyline odds are not available for this game")
			}
			option1 = homeTeam
			option2 = awayTeam
			odds1 = line.HomeTeamOdds.MoneyLine
			odds2 = line.AwayTeamOdds.MoneyLine
			spreadValue = nil
		} else {
			// ATS bet (default)
			lineValue := line.Spread
			// line must be on a 0.5 value to avoid pushes
			if lineValue == math.Trunc(lineValue) {
				lineValue += 0.5
			}
			option1 = fmt.Sprintf("%s %s", homeTeam, common.FormatOdds(lineValue))
			option2 = fmt.Sprintf("%s %s", awayTeam, common.FormatOdds(lineValue*-1))

			// Use actual spread odds if available, otherwise default to -110
			if line.HomeTeamOdds.SpreadOdds != 0 {
				odds1 = int(line.HomeTeamOdds.SpreadOdds)
			} else {
				odds1 = -110
			}
			if line.AwayTeamOdds.SpreadOdds != 0 {
				odds2 = int(line.AwayTeamOdds.SpreadOdds)
			} else {
				odds2 = -110
			}
			spreadValue = &lineValue
		}

		dbBet = models.Bet{
			Description:   fmt.Sprintf("%s @ %s (%s)\n- Broadcast: %s", awayTeam, homeTeam, formattedTime, cbbEvent.Competitions[0].Broadcast),
			Option1:       option1,
			Option2:       option2,
			Odds1:         odds1,
			Odds2:         odds2,
			Active:        true,
			GuildID:       guildID,
			ChannelID:     i.ChannelID,
			GameStartDate: &utcTime,
			EspnID:        &espnID,
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
		Title:       fmt.Sprintf("📢 New CBB %s Bet Created (Will Auto Close & Resolve)", betTypeLabel),
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

// AutoCreateCBBBet automatically creates an ATS bet for a subscribed team game.
// This function ONLY creates ATS bets - Moneyline bets must be created manually via slash command.
// It checks for existing ATS bets only (spread IS NOT NULL) to avoid conflicts with manually created Moneyline bets.
func AutoCreateCBBBet(s *discordgo.Session, db *gorm.DB, guildId string, channelId, gameId string) error {
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
		Where("espn_id = ? AND paid = 0 AND guild_id = ? AND spread IS NOT NULL", gameId, guildId).
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
			isHome := false
			if line.HomeTeamOdds.Team.Ref != "" {
				if strings.Contains(line.HomeTeamOdds.Team.Ref, fmt.Sprintf("/teams/%s?", competitor.ID)) {
					isHome = true
				}
			} else if competitor.HomeAway == "home" {
				isHome = true
			}

			if isHome {
				homeTeam = competitor.Team.ShortDisplayName
			} else {
				awayTeam = competitor.Team.ShortDisplayName
			}
		}

		fmt.Println(cbbEvent.Date)
		espnDateLayout := "2006-01-02T15:04Z"
		utcTime, err := time.Parse(espnDateLayout, cbbEvent.Date)
		if err != nil {
			return fmt.Errorf("err parsing time: %v", err)
		}

		// Convert to Eastern Time
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			return fmt.Errorf("err converting time: %v", err)
		}
		t := utcTime.In(loc)
		formattedTime := t.Format("Mon 03:04 pm MST")

		// line must be on a 0.5 value to avoid pushes
		lineValue := line.Spread
		if lineValue == math.Trunc(lineValue) {
			lineValue += 0.5
		}

		// Use actual spread odds from API if available, otherwise default to -110
		odds1 := -110
		odds2 := -110
		if line.HomeTeamOdds.SpreadOdds != 0 {
			odds1 = int(line.HomeTeamOdds.SpreadOdds)
		}
		if line.AwayTeamOdds.SpreadOdds != 0 {
			odds2 = int(line.AwayTeamOdds.SpreadOdds)
		}

		dbBet = models.Bet{
			Description:   fmt.Sprintf("%s @ %s (%s)\n- Broadcast: %s", awayTeam, homeTeam, formattedTime, cbbEvent.Competitions[0].Broadcast),
			Option1:       fmt.Sprintf("%s %s", homeTeam, common.FormatOdds(lineValue)),
			Option2:       fmt.Sprintf("%s %s", awayTeam, common.FormatOdds(lineValue*-1)),
			Odds1:         odds1,
			Odds2:         odds2,
			Active:        true,
			GuildID:       guildId,
			ChannelID:     channelId,
			GameStartDate: &utcTime,
			EspnID:        &gameId,
			AdminCreated:  true,
			Spread:        &lineValue,
		}
		db.Create(&dbBet)

		buttons := messageService.GetBetOnlyButtonsList(dbBet.Option1, dbBet.Option2, dbBet.ID)
		embed := &discordgo.MessageEmbed{
			Title:       "📢 New CBB Bet Created (Will Auto Close & Resolve)",
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
