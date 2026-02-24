package betService

import (
	"fmt"
	"log"
	"math"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"strconv"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

type ParlaySelection struct {
	BetIDs          []uint
	SelectedOptions map[uint]int
	MessageID       string
	ChannelID       string
}

var (
	parlaySelectionsMap = make(map[string]*ParlaySelection)
	parlaySelectionsMu  sync.RWMutex
)

func GetParlaySelection(sessionID string) (*ParlaySelection, bool) {
	parlaySelectionsMu.RLock()
	defer parlaySelectionsMu.RUnlock()
	selection, exists := parlaySelectionsMap[sessionID]
	return selection, exists
}

func StoreParlaySelection(sessionID string, selection *ParlaySelection) {
	parlaySelectionsMu.Lock()
	defer parlaySelectionsMu.Unlock()
	parlaySelectionsMap[sessionID] = selection
}

func CleanupParlaySelection(sessionID string) {
	parlaySelectionsMu.Lock()
	defer parlaySelectionsMu.Unlock()
	delete(parlaySelectionsMap, sessionID)
}

func createProgressBar(selected, total int) string {
	if total == 0 {
		return "▱▱▱▱▱▱▱▱▱▱ 0%"
	}

	filled := int(float64(selected) / float64(total) * 10)
	if filled > 10 {
		filled = 10
	}

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "▰"
	}
	for i := filled; i < 10; i++ {
		bar += "▱"
	}

	percentage := int(float64(selected) / float64(total) * 100)
	return fmt.Sprintf("%s %d%% (%d/%d)", bar, percentage, selected, total)
}

func CreateParlaySelector(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	var openBets []models.Bet

	result := db.Where("active = ? AND paid = ? AND guild_id = ?", true, false, i.GuildID).Find(&openBets)
	if result.Error != nil {
		common.SendError(s, i, result.Error, db)
		return
	}

	if len(openBets) < 2 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You need at least 2 open bets to create a parlay. Currently there are " + strconv.Itoa(len(openBets)) + " open bet(s).",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	var selectOptions []discordgo.SelectMenuOption
	for _, bet := range openBets {
		label := bet.Description
		if len(label) > 100 {
			label = label[:97] + "..."
		}

		description := fmt.Sprintf("%s | %s", bet.Option1, bet.Option2)
		if len(description) > 100 {
			description = description[:97] + "..."
		}

		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label:       label,
			Value:       strconv.Itoa(int(bet.ID)),
			Description: description,
		})
	}

	sessionID := i.Interaction.ID

	minValues := 2
	maxValues := len(openBets)
	if maxValues > 10 {
		maxValues = 10
	}

	content := fmt.Sprintf("Select **2-%d** bets to include in your parlay (you'll choose options for each bet next):", maxValues)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("parlay_select_bets_%s", sessionID),
							Placeholder: "Select bets for parlay (min 2)",
							MinValues:   &minValues,
							MaxValues:   maxValues,
							Options:     selectOptions,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Cancel",
							CustomID: fmt.Sprintf("parlay_cancel_%s", sessionID),
							Style:    discordgo.DangerButton,
						},
					},
				},
			},
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
	}
}

func HandleParlayBetSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	sessionID := strings.TrimPrefix(customID, "parlay_select_bets_")
	selectedBetIDs := i.MessageComponentData().Values

	if len(selectedBetIDs) < 2 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You must select at least 2 bets for a parlay.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	var bets []models.Bet
	var betIDs []uint
	for _, idStr := range selectedBetIDs {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		betIDs = append(betIDs, uint(id))
	}

	result := db.Where("id IN ? AND active = ? AND paid = ? AND guild_id = ?", betIDs, true, false, i.GuildID).Find(&bets)
	if result.Error != nil || len(bets) != len(selectedBetIDs) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Some selected bets are no longer available. Please try again.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	selection := &ParlaySelection{
		BetIDs:          betIDs,
		SelectedOptions: make(map[uint]int),
	}
	StoreParlaySelection(sessionID, selection)

	var fields []*discordgo.MessageEmbedField
	for idx, bet := range bets {
		fieldName := fmt.Sprintf("Bet %d", idx+1)
		if len(bet.Description) > 60 {
			fieldValue := fmt.Sprintf("**%s**\n1️⃣ %s (%s) | 2️⃣ %s (%s)",
				bet.Description[:57]+"...",
				bet.Option1, common.FormatOdds(float64(bet.Odds1)),
				bet.Option2, common.FormatOdds(float64(bet.Odds2)))
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   fieldName,
				Value:  fieldValue,
				Inline: true,
			})
		} else {
			fieldValue := fmt.Sprintf("**%s**\n1️⃣ %s (%s)\n2️⃣ %s (%s)",
				bet.Description,
				bet.Option1, common.FormatOdds(float64(bet.Odds1)),
				bet.Option2, common.FormatOdds(float64(bet.Odds2)))
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   fieldName,
				Value:  fieldValue,
				Inline: true,
			})
		}
	}

	progressBar := createProgressBar(0, len(bets))
	description := fmt.Sprintf("%s\n\n📋 Select an option for each of the **%d** bets below:", progressBar, len(bets))

	embed := &discordgo.MessageEmbed{
		Title:       "🎯 Create Parlay",
		Description: description,
		Fields:      fields,
		Color:       0x5865F2,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Click the buttons below to select your picks • All selections must be made before submitting",
		},
	}

	var actionRows []discordgo.MessageComponent
	var currentRowComponents []discordgo.MessageComponent

	for idx, bet := range bets {
		if len(currentRowComponents) >= 4 {
			actionRows = append(actionRows, discordgo.ActionsRow{Components: currentRowComponents})
			currentRowComponents = []discordgo.MessageComponent{}
		}

		betIDStr := strconv.Itoa(int(bet.ID))

		label1 := bet.Option1
		if len(label1) > 70 {
			label1 = label1[:67] + "..."
		}
		label2 := bet.Option2
		if len(label2) > 70 {
			label2 = label2[:67] + "..."
		}

		currentRowComponents = append(currentRowComponents,
			discordgo.Button{
				Label:    fmt.Sprintf("%d️⃣ %s", idx+1, label1),
				CustomID: fmt.Sprintf("parlay_option_%s_%s_1", sessionID, betIDStr),
				Style:    discordgo.PrimaryButton,
			},
			discordgo.Button{
				Label:    fmt.Sprintf("%d️⃣ %s", idx+1, label2),
				CustomID: fmt.Sprintf("parlay_option_%s_%s_2", sessionID, betIDStr),
				Style:    discordgo.SecondaryButton,
			},
		)
	}

	if len(currentRowComponents) > 0 {
		actionRows = append(actionRows, discordgo.ActionsRow{Components: currentRowComponents})
	}

	actionRows = append(actionRows, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Submit Parlay",
				CustomID: fmt.Sprintf("parlay_submit_%s", sessionID),
				Style:    discordgo.SuccessButton,
				Disabled: true,
			},
			discordgo.Button{
				Label:    "Cancel",
				CustomID: fmt.Sprintf("parlay_cancel_%s", sessionID),
				Style:    discordgo.DangerButton,
			},
		},
	})

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: actionRows,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func HandleParlayOptionSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	parts := strings.Split(customID, "_")
	if len(parts) != 5 {
		return fmt.Errorf("invalid parlay option custom ID format")
	}

	sessionID := parts[2]
	betIDStr := parts[3]
	optionStr := parts[4]

	betID, err := strconv.Atoi(betIDStr)
	if err != nil {
		return err
	}

	option, err := strconv.Atoi(optionStr)
	if err != nil || (option != 1 && option != 2) {
		return fmt.Errorf("invalid option value")
	}

	selection, exists := GetParlaySelection(sessionID)
	if !exists {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Parlay session expired. Please start over.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	if selection.MessageID == "" && i.Message != nil {
		selection.MessageID = i.Message.ID
		selection.ChannelID = i.ChannelID
	}

	selection.SelectedOptions[uint(betID)] = option
	StoreParlaySelection(sessionID, selection)

	allSelected := len(selection.SelectedOptions) == len(selection.BetIDs)

	var bet models.Bet
	db.First(&bet, betID)

	optionName := bet.Option1
	if option == 2 {
		optionName = bet.Option2
	}

	var betFields []*discordgo.MessageEmbedField
	var actionRows []discordgo.MessageComponent
	var currentRowComponents []discordgo.MessageComponent

	var bets []models.Bet
	db.Where("id IN ?", selection.BetIDs).Find(&bets)

	for idx, bet := range bets {
		selectedOption, hasSelection := selection.SelectedOptions[bet.ID]

		var fieldValue string
		fieldName := fmt.Sprintf("Bet %d", idx+1)

		if hasSelection {
			selectedOptionName := bet.Option1
			if selectedOption == 2 {
				selectedOptionName = bet.Option2
			}
			odds := bet.Odds1
			if selectedOption == 2 {
				odds = bet.Odds2
			}

			if len(bet.Description) > 60 {
				fieldValue = fmt.Sprintf("**%s**\n✅ **%s** (%s)\n⚪ %s (%s)",
					bet.Description[:57]+"...",
					selectedOptionName, common.FormatOdds(float64(odds)),
					bet.Option2, common.FormatOdds(float64(bet.Odds2)))
				if selectedOption == 2 {
					fieldValue = fmt.Sprintf("**%s**\n✅ **%s** (%s)\n⚪ %s (%s)",
						bet.Description[:57]+"...",
						selectedOptionName, common.FormatOdds(float64(odds)),
						bet.Option1, common.FormatOdds(float64(bet.Odds1)))
				}
			} else {
				fieldValue = fmt.Sprintf("**%s**\n✅ **%s** (%s)\n⚪ %s (%s)",
					bet.Description,
					selectedOptionName, common.FormatOdds(float64(odds)),
					bet.Option2, common.FormatOdds(float64(bet.Odds2)))
				if selectedOption == 2 {
					fieldValue = fmt.Sprintf("**%s**\n✅ **%s** (%s)\n⚪ %s (%s)",
						bet.Description,
						selectedOptionName, common.FormatOdds(float64(odds)),
						bet.Option1, common.FormatOdds(float64(bet.Odds1)))
				}
			}
		} else {
			if len(bet.Description) > 60 {
				fieldValue = fmt.Sprintf("**%s**\n1️⃣ %s (%s)\n2️⃣ %s (%s)",
					bet.Description[:57]+"...",
					bet.Option1, common.FormatOdds(float64(bet.Odds1)),
					bet.Option2, common.FormatOdds(float64(bet.Odds2)))
			} else {
				fieldValue = fmt.Sprintf("**%s**\n1️⃣ %s (%s)\n2️⃣ %s (%s)",
					bet.Description,
					bet.Option1, common.FormatOdds(float64(bet.Odds1)),
					bet.Option2, common.FormatOdds(float64(bet.Odds2)))
			}
		}

		betFields = append(betFields, &discordgo.MessageEmbedField{
			Name:   fieldName,
			Value:  fieldValue,
			Inline: true,
		})

		if len(currentRowComponents) >= 4 {
			actionRows = append(actionRows, discordgo.ActionsRow{Components: currentRowComponents})
			currentRowComponents = []discordgo.MessageComponent{}
		}

		betIDStr := strconv.Itoa(int(bet.ID))
		label1 := bet.Option1
		if len(label1) > 70 {
			label1 = label1[:67] + "..."
		}
		label2 := bet.Option2
		if len(label2) > 70 {
			label2 = label2[:67] + "..."
		}

		style1 := discordgo.PrimaryButton
		style2 := discordgo.SecondaryButton
		if hasSelection && selectedOption == 1 {
			style1 = discordgo.SuccessButton
		} else if hasSelection && selectedOption == 2 {
			style2 = discordgo.SuccessButton
		}

		currentRowComponents = append(currentRowComponents,
			discordgo.Button{
				Label:    fmt.Sprintf("%d️⃣ %s", idx+1, label1),
				CustomID: fmt.Sprintf("parlay_option_%s_%s_1", sessionID, betIDStr),
				Style:    style1,
			},
			discordgo.Button{
				Label:    fmt.Sprintf("%d️⃣ %s", idx+1, label2),
				CustomID: fmt.Sprintf("parlay_option_%s_%s_2", sessionID, betIDStr),
				Style:    style2,
			},
		)
	}

	if len(currentRowComponents) > 0 {
		actionRows = append(actionRows, discordgo.ActionsRow{Components: currentRowComponents})
	}

	actionRows = append(actionRows, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Submit Parlay",
				CustomID: fmt.Sprintf("parlay_submit_%s", sessionID),
				Style:    discordgo.SuccessButton,
				Disabled: !allSelected,
			},
			discordgo.Button{
				Label:    "Cancel",
				CustomID: fmt.Sprintf("parlay_cancel_%s", sessionID),
				Style:    discordgo.DangerButton,
			},
		},
	})

	progressBar := createProgressBar(len(selection.SelectedOptions), len(selection.BetIDs))

	var description string
	var embedColor int
	if allSelected {
		description = fmt.Sprintf("%s\n\n✅ **All selections complete!** Ready to submit your parlay.\n\nLast selection: **%s**", progressBar, optionName)
		embedColor = 0x57F287
	} else {
		description = fmt.Sprintf("%s\n\n📝 **Selected:** %s (Bet #%d)\n\n⏳ Continue selecting options for the remaining bets...",
			progressBar, optionName, betID)
		embedColor = 0x5865F2
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🎯 Create Parlay",
		Description: description,
		Fields:      betFields,
		Color:       embedColor,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("%d/%d selections complete", len(selection.SelectedOptions), len(selection.BetIDs)),
		},
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: actionRows,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func HandleParlaySubmit(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	sessionID := strings.TrimPrefix(customID, "parlay_submit_")

	selection, exists := GetParlaySelection(sessionID)
	if !exists {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Parlay session expired. Please start over.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	if i.Message != nil {
		selection.MessageID = i.Message.ID
		selection.ChannelID = i.ChannelID
		StoreParlaySelection(sessionID, selection)
	}

	if len(selection.SelectedOptions) != len(selection.BetIDs) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Please select options for all bets before submitting.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: i.Member.User.ID, GuildID: i.GuildID})
	if result.Error != nil {
		return result.Error
	}

	var bets []models.Bet
	db.Where("id IN ?", selection.BetIDs).Find(&bets)

	var oddsList []int
	for _, bet := range bets {
		option := selection.SelectedOptions[bet.ID]
		odds := common.GetOddsFromBet(bet, option)
		oddsList = append(oddsList, odds)
	}

	oddsMultiplier := common.CalculateParlayOddsMultiplier(oddsList)
	totalOddsStr := fmt.Sprintf("%.2fx", oddsMultiplier)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    fmt.Sprintf("Enter Parlay Amount (Odds: %s)", totalOddsStr),
			CustomID: fmt.Sprintf("parlay_amount_%s", sessionID),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "parlay_amount_input",
							Label:       fmt.Sprintf("Bet Amount (Max: %.0f)", math.Floor(user.Points)),
							Style:       discordgo.TextInputShort,
							Placeholder: "Enter amount",
							Required:    true,
						},
					},
				},
			},
		},
	})

	return err
}

func HandleParlayAmount(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	sessionID := strings.TrimPrefix(customID, "parlay_amount_")

	selection, exists := GetParlaySelection(sessionID)
	if !exists {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Parlay session expired. Please start over.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	amountStr := i.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	amount, err := strconv.Atoi(amountStr)
	if err != nil || amount <= 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Invalid bet amount. Please enter a positive number.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: i.Member.User.ID, GuildID: i.GuildID})
	if result.Error != nil {
		return result.Error
	}

	if user.Points < float64(amount) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You do not have enough points to place this parlay.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	var bets []models.Bet
	result = db.Where("id IN ? AND active = ? AND paid = ? AND guild_id = ?", selection.BetIDs, true, false, i.GuildID).Find(&bets)
	if result.Error != nil || len(bets) != len(selection.BetIDs) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Some selected bets are no longer available. Please create a new parlay.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		CleanupParlaySelection(sessionID)
		return nil
	}

	var oddsList []int
	for _, bet := range bets {
		option := selection.SelectedOptions[bet.ID]
		odds := common.GetOddsFromBet(bet, option)
		oddsList = append(oddsList, odds)
	}

	oddsMultiplier := common.CalculateParlayOddsMultiplier(oddsList)

	parlay := models.Parlay{
		UserID:        user.ID,
		GuildID:       i.GuildID,
		Amount:        amount,
		TotalOdds:     oddsMultiplier,
		Status:        "pending",
		ParlayEntries: []models.ParlayEntry{},
	}

	result = db.Create(&parlay)
	if result.Error != nil {
		return result.Error
	}

	for _, bet := range bets {
		option := selection.SelectedOptions[bet.ID]
		parlayEntry := models.ParlayEntry{
			ParlayID:       parlay.ID,
			BetID:          bet.ID,
			SelectedOption: option,
			Spread:         bet.Spread,
			Resolved:       false,
			Won:            nil,
		}
		db.Create(&parlayEntry)
	}

	user.Points -= float64(amount)
	db.Save(&user)

	potentialPayout := common.CalculateParlayPayout(amount, oddsMultiplier)

	var summary strings.Builder
	summary.WriteString("**Parlay Created Successfully!**\n\n")
	for idx, bet := range bets {
		option := selection.SelectedOptions[bet.ID]
		optionName := bet.Option1
		if option == 2 {
			optionName = bet.Option2
		}
		summary.WriteString(fmt.Sprintf("%d. %s: **%s**\n", idx+1, bet.Description, optionName))
	}
	summary.WriteString(fmt.Sprintf("\n**Amount:** %d points\n", amount))
	summary.WriteString(fmt.Sprintf("**Combined Odds:** %.2fx\n", oddsMultiplier))
	summary.WriteString(fmt.Sprintf("**Potential Payout:** %.1f points\n", potentialPayout))
	summary.WriteString(fmt.Sprintf("**Remaining Points:** %.1f", user.Points))

	successEmbed := &discordgo.MessageEmbed{
		Title:       "✅ Parlay Placed Successfully",
		Description: summary.String(),
		Color:       0x00ff00,
	}

	if selection.MessageID != "" && selection.ChannelID != "" {
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
			},
		})
		if err == nil {
			_, editErr := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:         selection.MessageID,
				Channel:    selection.ChannelID,
				Embeds:     &[]*discordgo.MessageEmbed{successEmbed},
				Components: &[]discordgo.MessageComponent{},
			})
			if editErr != nil {
				_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Embeds: []*discordgo.MessageEmbed{successEmbed},
				})
			} else {
				_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: "✅ Parlay created successfully!",
				})
			}
		} else {
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{successEmbed},
					Flags:  discordgo.MessageFlagsEphemeral,
				},
			})
		}
	} else {
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{successEmbed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
	}

	CleanupParlaySelection(sessionID)

	if err != nil {
		return err
	}

	return nil
}

func HandleParlayCancel(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	sessionID := strings.TrimPrefix(customID, "parlay_cancel_")
	CleanupParlaySelection(sessionID)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "❌ Parlay creation cancelled.",
			Components: []discordgo.MessageComponent{},
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})

	return err
}

func UpdateParlaysOnBetResolution(s *discordgo.Session, db *gorm.DB, betID uint, winningOption int, scoreDiff int) error {
	var parlayEntries []models.ParlayEntry
	result := db.Where("bet_id = ? AND resolved = ?", betID, false).Find(&parlayEntries)
	if result.Error != nil {
		return result.Error
	}

	var bet models.Bet
	if err := db.First(&bet, betID).Error; err != nil {
		return err
	}

	for _, entry := range parlayEntries {
		var parlay models.Parlay
		db.Preload("ParlayEntries").Preload("ParlayEntries.Bet").First(&parlay, entry.ParlayID)
		previousStatus := parlay.Status

		allResolved := true
		hasLoss := false
		for _, pe := range parlay.ParlayEntries {
			if pe.ID == entry.ID {
				continue
			}

			if !pe.Resolved {
				allResolved = false
			}
			if pe.Won != nil && !*pe.Won {
				hasLoss = true
			}
		}
		if hasLoss {
			entry.Resolved = true
			db.Save(&entry)
			continue
		}

		var won bool
		if bet.Spread == nil {
			if scoreDiff == 0 {
				won = false
			} else {
				won = entry.SelectedOption == winningOption
			}
		} else {
			if scoreDiff == 0 {
				won = entry.SelectedOption == winningOption
			} else {
				var entrySpread float64
				if entry.Spread != nil {
					entrySpread = *entry.Spread
				} else {
					entrySpread = *bet.Spread
				}
				won = common.CalculateBetEntryWin(entry.SelectedOption, scoreDiff, entrySpread)
			}
		}
		entry.Resolved = true

		entry.Won = &won
		db.Save(&entry)

		if !won {
			parlay.Status = "lost"
			db.Save(&parlay)

			if previousStatus != "lost" && previousStatus != "won" {
				var user models.User
				db.First(&user, parlay.UserID)
				user.TotalBetsLost++
				user.TotalPointsLost += float64(parlay.Amount)
				db.Save(&user)

				guild, guildErr := guildService.GetGuildInfo(s, db, parlay.GuildID, "")
				if guildErr == nil {
					db.Model(&models.Guild{}).Where("id = ?", guild.ID).UpdateColumn("pool", gorm.Expr("pool + ?", float64(parlay.Amount)))
				}
				SendParlayResolutionNotification(s, db, parlay, false)
			}
		} else if allResolved {
			if !hasLoss {
				parlay.Status = "won"
				db.Save(&parlay)

				var user models.User
				db.First(&user, parlay.UserID)
				payout := common.CalculateParlayPayout(parlay.Amount, parlay.TotalOdds)
				var err error
				payout, _, err = cardService.ApplyHeismanCampaignIfApplicable(db, user, payout)
				if err != nil {
					log.Printf("Error applying Heisman Campaign card for parlay ID %d, user ID %d, payout %.2f: %v", parlay.ID, parlay.UserID, payout, err)
					return fmt.Errorf("failed to apply Heisman Campaign card for parlay %d (user %d): %w", parlay.ID, parlay.UserID, err)
				}
				user.Points += payout
				user.TotalBetsWon++
				user.TotalPointsWon += payout
				db.Save(&user)

				if previousStatus != "lost" && previousStatus != "won" {
					SendParlayResolutionNotification(s, db, parlay, true, payout)
				}
			}
		} else {
			parlay.Status = "partial"
			db.Save(&parlay)
		}
	}

	return nil
}

func SendParlayResolutionNotification(s *discordgo.Session, db *gorm.DB, parlay models.Parlay, won bool, actualPayoutWhenWon ...float64) {
	guild, err := guildService.GetGuildInfo(s, db, parlay.GuildID, "")
	if err != nil || guild.BetChannelID == "" {
		return
	}

	var user models.User
	db.First(&user, parlay.UserID)
	if user.ID == 0 {
		return
	}

	var title string
	var color int
	var description strings.Builder

	if won {
		title = "🎉 Parlay Hit!"
		color = 0x57F287
		payout := common.CalculateParlayPayout(parlay.Amount, parlay.TotalOdds)
		if len(actualPayoutWhenWon) > 0 {
			payout = actualPayoutWhenWon[0]
		}
		description.WriteString(fmt.Sprintf("<@%s> Your parlay has been **won**!\n\n", user.DiscordID))
		description.WriteString(fmt.Sprintf("**Amount Wagered:** %d points\n", parlay.Amount))
		description.WriteString(fmt.Sprintf("**Combined Odds:** %.2fx\n", parlay.TotalOdds))
		description.WriteString(fmt.Sprintf("**Payout:** %.1f points\n", payout))
	} else {
		title = "💔 Parlay Lost"
		color = 0xED4245
		description.WriteString(fmt.Sprintf("<@%s> Your parlay has been **lost**.\n\n", user.DiscordID))
		description.WriteString(fmt.Sprintf("**Amount Wagered:** %d points\n", parlay.Amount))
		description.WriteString(fmt.Sprintf("**Combined Odds:** %.2fx\n", parlay.TotalOdds))
	}

	description.WriteString("\n**Parlay Details:**\n")
	for idx, entry := range parlay.ParlayEntries {
		optionName := entry.Bet.Option1
		if entry.SelectedOption == 2 {
			optionName = entry.Bet.Option2
		}

		status := "✅ Won"
		if entry.Won != nil && !*entry.Won {
			status = "❌ Lost"
		} else if !entry.Resolved {
			status = "⏳ Pending"
		}

		description.WriteString(fmt.Sprintf("%d. %s: **%s** - %s\n", idx+1, entry.Bet.Description, optionName, status))
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description.String(),
		Color:       color,
	}

	_, err = s.ChannelMessageSendComplex(guild.BetChannelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		fmt.Printf("Error sending parlay resolution notification: %v\n", err)
	}
}

func MyParlays(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: i.Member.User.ID, GuildID: i.GuildID})
	if result.Error != nil {
		common.SendError(s, i, result.Error, db)
		return
	}

	var parlays []models.Parlay
	result = db.Preload("ParlayEntries").Preload("ParlayEntries.Bet").
		Where("user_id = ? AND guild_id = ? AND status IN ?", user.ID, i.GuildID, []string{"pending", "partial"}).
		Find(&parlays)

	if result.Error != nil {
		common.SendError(s, i, result.Error, db)
		return
	}

	if len(parlays) == 0 {
		embed := &discordgo.MessageEmbed{
			Title:       "🎯 Your Active Parlays",
			Description: "You have no active parlays.",
			Color:       0x5865F2,
		}
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	var embeds []*discordgo.MessageEmbed

	for parlayIdx, parlay := range parlays {
		potentialPayout := common.CalculateParlayPayout(parlay.Amount, parlay.TotalOdds)

		description := fmt.Sprintf("**Amount:** %d points\n**Odds:** %.2fx\n**Potential Payout:** %.1f points", parlay.Amount, parlay.TotalOdds, potentialPayout)

		var fields []*discordgo.MessageEmbedField
		for entryIdx, entry := range parlay.ParlayEntries {
			optionName := entry.Bet.Option1
			if entry.SelectedOption == 2 {
				optionName = entry.Bet.Option2
			}

			status := "⏳ Pending"
			if entry.Resolved {
				if entry.Won != nil && *entry.Won {
					status = "✅ Won"
				} else {
					status = "❌ Lost"
				}
			}

			fieldValue := fmt.Sprintf("**%s**\n%s", optionName, status)
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("Leg %d: %s", entryIdx+1, entry.Bet.Description),
				Value:  fieldValue,
				Inline: false,
			})
		}

		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("🎯 Parlay #%d", parlayIdx+1),
			Description: description,
			Fields:      fields,
			Color:       0x5865F2,
		}

		embeds = append(embeds, embed)
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("You have %d active parlay(s):", len(parlays)),
			Embeds:  embeds,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
	}
}
