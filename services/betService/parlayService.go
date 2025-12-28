package betService

import (
	"fmt"
	"math"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"strconv"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

// ParlaySelection stores the current state of a parlay being created
type ParlaySelection struct {
	BetIDs          []uint
	SelectedOptions map[uint]int // betID -> option (1 or 2)
}

var (
	parlaySelectionsMap = make(map[string]*ParlaySelection)
	parlaySelectionsMu  sync.RWMutex
)

// GetParlaySelection retrieves parlay selection for a given session ID
func GetParlaySelection(sessionID string) (*ParlaySelection, bool) {
	parlaySelectionsMu.RLock()
	defer parlaySelectionsMu.RUnlock()
	selection, exists := parlaySelectionsMap[sessionID]
	return selection, exists
}

// StoreParlaySelection stores parlay selection for a given session ID
func StoreParlaySelection(sessionID string, selection *ParlaySelection) {
	parlaySelectionsMu.Lock()
	defer parlaySelectionsMu.Unlock()
	parlaySelectionsMap[sessionID] = selection
}

// CleanupParlaySelection removes parlay selection for a given session ID
func CleanupParlaySelection(sessionID string) {
	parlaySelectionsMu.Lock()
	defer parlaySelectionsMu.Unlock()
	delete(parlaySelectionsMap, sessionID)
}

// createProgressBar creates a visual progress bar using emoji
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

// CreateParlaySelector shows available open bets that can be added to a parlay
func CreateParlaySelector(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	var openBets []models.Bet

	// Get all active, unpaid bets in the guild
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

	// Generate unique session ID from interaction ID
	sessionID := i.Interaction.ID

	// Store the session data temporarily (we'll use a simple map for now, but in production you might want Redis)
	// For now, we'll use the interaction ID as part of the custom ID

	minValues := 2 // Minimum 2 bets for a parlay
	maxValues := len(openBets)
	if maxValues > 10 {
		maxValues = 10 // Discord limit is 25, but we'll limit to 10 for usability
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

// HandleParlayBetSelection handles when user selects which bets to include in parlay
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

	// Fetch the selected bets
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

	// Store the selection in memory
	selection := &ParlaySelection{
		BetIDs:          betIDs,
		SelectedOptions: make(map[uint]int),
	}
	StoreParlaySelection(sessionID, selection)

	// Create embed showing bets and their options
	var fields []*discordgo.MessageEmbedField
	for idx, bet := range bets {
		// Use compact inline format
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

	// Progress indicator
	progressBar := createProgressBar(0, len(bets))
	description := fmt.Sprintf("%s\n\n📋 Select an option for each of the **%d** bets below:", progressBar, len(bets))

	embed := &discordgo.MessageEmbed{
		Title:       "🎯 Create Parlay",
		Description: description,
		Fields:      fields,
		Color:       0x5865F2, // Discord blurple
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Click the buttons below to select your picks • All selections must be made before submitting",
		},
	}

	// Create buttons for each bet to select option
	var actionRows []discordgo.MessageComponent
	var currentRowComponents []discordgo.MessageComponent

	for idx, bet := range bets {
		// Start a new row if current row has 5 components (2 buttons per bet = 2.5 bets per row, so use 2)
		if len(currentRowComponents) >= 4 {
			actionRows = append(actionRows, discordgo.ActionsRow{Components: currentRowComponents})
			currentRowComponents = []discordgo.MessageComponent{}
		}

		betIDStr := strconv.Itoa(int(bet.ID))

		// Truncate option names for button labels - more compact
		label1 := bet.Option1
		if len(label1) > 70 {
			label1 = label1[:67] + "..."
		}
		label2 := bet.Option2
		if len(label2) > 70 {
			label2 = label2[:67] + "..."
		}

		// Use emoji prefix for better visual distinction
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

	// Add remaining buttons
	if len(currentRowComponents) > 0 {
		actionRows = append(actionRows, discordgo.ActionsRow{Components: currentRowComponents})
	}

	// Add submit and cancel buttons
	actionRows = append(actionRows, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Submit Parlay",
				CustomID: fmt.Sprintf("parlay_submit_%s", sessionID),
				Style:    discordgo.SuccessButton,
				Disabled: true, // Disabled until all options are selected
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

// HandleParlayOptionSelection handles when user selects an option for a specific bet in the parlay
func HandleParlayOptionSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	// Parse custom ID: parlay_option_{sessionID}_{betID}_{option}
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

	// Get the selection from memory
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

	// Update the selection
	selection.SelectedOptions[uint(betID)] = option
	StoreParlaySelection(sessionID, selection)

	// Check if all options are selected
	allSelected := len(selection.SelectedOptions) == len(selection.BetIDs)

	// Get the bet to show which option was selected
	var bet models.Bet
	db.First(&bet, betID)

	optionName := bet.Option1
	if option == 2 {
		optionName = bet.Option2
	}

	// Update the message to show progress
	var betFields []*discordgo.MessageEmbedField
	var actionRows []discordgo.MessageComponent
	var currentRowComponents []discordgo.MessageComponent

	// Fetch all bets
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

			// Compact format with selected option highlighted
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
			// Unselected format
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

		// Create buttons
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

	// Add submit and cancel buttons
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

	// Create progress bar and description
	progressBar := createProgressBar(len(selection.SelectedOptions), len(selection.BetIDs))

	var description string
	var embedColor int
	if allSelected {
		description = fmt.Sprintf("%s\n\n✅ **All selections complete!** Ready to submit your parlay.\n\nLast selection: **%s**", progressBar, optionName)
		embedColor = 0x57F287 // Discord green
	} else {
		description = fmt.Sprintf("%s\n\n📝 **Selected:** %s (Bet #%d)\n\n⏳ Continue selecting options for the remaining bets...",
			progressBar, optionName, betID)
		embedColor = 0x5865F2 // Discord blurple
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

// HandleParlaySubmit handles submitting the parlay and asking for bet amount
func HandleParlaySubmit(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	sessionID := strings.TrimPrefix(customID, "parlay_submit_")

	// Get the selection
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

	// Verify all options are selected
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

	// Get user and check points
	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: i.Member.User.ID, GuildID: i.GuildID})
	if result.Error != nil {
		return result.Error
	}

	// Fetch bets to calculate odds
	var bets []models.Bet
	db.Where("id IN ?", selection.BetIDs).Find(&bets)

	// Calculate combined odds
	var oddsList []int
	for _, bet := range bets {
		option := selection.SelectedOptions[bet.ID]
		odds := common.GetOddsFromBet(bet, option)
		oddsList = append(oddsList, odds)
	}

	oddsMultiplier := common.CalculateParlayOddsMultiplier(oddsList)
	totalOddsStr := fmt.Sprintf("%.2fx", oddsMultiplier)

	// Show modal for bet amount (summary removed since it was editable)
	// Combined odds included in title for reference
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

// HandleParlayAmount handles the modal submission with parlay amount
func HandleParlayAmount(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	sessionID := strings.TrimPrefix(customID, "parlay_amount_")

	// Get the selection
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

	// Get amount from modal
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

	// Get user
	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: i.Member.User.ID, GuildID: i.GuildID})
	if result.Error != nil {
		return result.Error
	}

	// Check if user has enough points
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

	// Verify bets are still active
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

	// Calculate combined odds
	var oddsList []int
	for _, bet := range bets {
		option := selection.SelectedOptions[bet.ID]
		odds := common.GetOddsFromBet(bet, option)
		oddsList = append(oddsList, odds)
	}

	oddsMultiplier := common.CalculateParlayOddsMultiplier(oddsList)

	// Create parlay
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

	// Create parlay entries
	for _, bet := range bets {
		option := selection.SelectedOptions[bet.ID]
		parlayEntry := models.ParlayEntry{
			ParlayID:       parlay.ID,
			BetID:          bet.ID,
			SelectedOption: option,
			Spread:         bet.Spread, // Store the spread at the time the parlay entry is created
			Resolved:       false,
			Won:            nil,
		}
		db.Create(&parlayEntry)
	}

	// Deduct points from user
	user.Points -= float64(amount)
	db.Save(&user)

	// Clean up session
	CleanupParlaySelection(sessionID)

	// Calculate potential payout
	potentialPayout := common.CalculateParlayPayout(amount, oddsMultiplier)

	// Build summary message
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

	embed := &discordgo.MessageEmbed{
		Title:       "✅ Parlay Placed Successfully",
		Description: summary.String(),
		Color:       0x00ff00,
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// HandleParlayCancel handles cancelling parlay creation
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

// UpdateParlaysOnBetResolution updates all parlays that include this bet when it resolves
func UpdateParlaysOnBetResolution(s *discordgo.Session, db *gorm.DB, betID uint, winningOption int, scoreDiff int) error {
	// Find all parlay entries for this bet that are not yet resolved
	var parlayEntries []models.ParlayEntry
	result := db.Where("bet_id = ? AND resolved = ?", betID, false).Find(&parlayEntries)
	if result.Error != nil {
		return result.Error
	}

	// Get the bet to check if it has a spread
	var bet models.Bet
	if err := db.First(&bet, betID).Error; err != nil {
		return err
	}

	for _, entry := range parlayEntries {
		// Mark entry as resolved
		entry.Resolved = true

		// Determine if this parlay entry won
		var won bool
		if bet.Spread == nil {
			// Moneyline bet: simple option comparison
			won = entry.SelectedOption == winningOption
		} else {
			// ATS bet
			if scoreDiff == 0 {
				// Manually resolved bet (no scoreDiff available): use simple option comparison
				// Note: This may not be accurate if parlay entries have different spreads,
				// but we can't calculate properly without the actual score difference
				won = entry.SelectedOption == winningOption
			} else {
				// Auto-resolved bet: use the parlay entry's spread (or fallback to bet spread for legacy entries)
				var entrySpread float64
				if entry.Spread != nil {
					entrySpread = *entry.Spread
				} else {
					// Fallback for legacy parlay entries that don't have spread stored
					entrySpread = *bet.Spread
				}
				won = common.CalculateBetEntryWin(entry.SelectedOption, scoreDiff, entrySpread)
			}
		}

		entry.Won = &won
		db.Save(&entry)

		// Get the parlay with previous status to detect when it becomes fully resolved
		var parlay models.Parlay
		db.Preload("ParlayEntries").Preload("ParlayEntries.Bet").First(&parlay, entry.ParlayID)
		previousStatus := parlay.Status

		// Check if any other entries are unresolved
		allResolved := true
		hasLoss := false
		for _, pe := range parlay.ParlayEntries {
			if !pe.Resolved {
				allResolved = false
				break
			}
			if pe.Won != nil && !*pe.Won {
				hasLoss = true
			}
		}

		// If this entry lost, mark parlay as lost immediately
		if !won {
			parlay.Status = "lost"
			db.Save(&parlay)

			// Update user stats - they already lost the bet amount when placing the parlay
			var user models.User
			db.First(&user, parlay.UserID)
			user.TotalBetsLost++
			user.TotalPointsLost += float64(parlay.Amount)
			db.Save(&user)

			// Add lost parlay amount to guild pool and send notification if parlay just became lost
			if previousStatus != "lost" && previousStatus != "won" {
				guild, guildErr := guildService.GetGuildInfo(s, db, parlay.GuildID, "")
				if guildErr == nil {
					// Add lost parlay amount to guild pool (atomic update to prevent race conditions)
					db.Model(&models.Guild{}).Where("id = ?", guild.ID).UpdateColumn("pool", gorm.Expr("pool + ?", float64(parlay.Amount)))
				}
				SendParlayResolutionNotification(s, db, parlay, false)
			}
		} else if allResolved {
			// All bets resolved - this entry won, and all other entries have also resolved
			// Since we didn't enter the !won branch above, this entry won
			// If hasLoss is true, it means a previous entry lost, but the parlay was already marked as lost
			// when that entry lost, so we don't need to do anything here
			// Only handle the case where all entries won
			if !hasLoss {
				// All bets won! Calculate and pay out parlay
				parlay.Status = "won"
				db.Save(&parlay)

				var user models.User
				db.First(&user, parlay.UserID)
				payout := common.CalculateParlayPayout(parlay.Amount, parlay.TotalOdds)
				user.Points += payout
				user.TotalBetsWon++
				user.TotalPointsWon += payout
				db.Save(&user)

				// Send notification if parlay just became fully resolved
				if previousStatus != "lost" && previousStatus != "won" {
					SendParlayResolutionNotification(s, db, parlay, true)
				}
			}
			// If hasLoss is true, the parlay was already marked as lost and notification sent when that entry lost
		} else {
			// Some bets still pending
			parlay.Status = "partial"
			db.Save(&parlay)
		}
	}

	return nil
}

// SendParlayResolutionNotification sends a message to the guild betting channel when a parlay is fully resolved
func SendParlayResolutionNotification(s *discordgo.Session, db *gorm.DB, parlay models.Parlay, won bool) {
	// Get guild info to find betting channel
	guild, err := guildService.GetGuildInfo(s, db, parlay.GuildID, "")
	if err != nil || guild.BetChannelID == "" {
		// If no betting channel set, skip notification
		return
	}

	// Get user who placed the parlay
	var user models.User
	db.First(&user, parlay.UserID)
	if user.ID == 0 {
		return
	}

	// Build embed message
	var title string
	var color int
	var description strings.Builder

	if won {
		title = "🎉 Parlay Hit!"
		color = 0x57F287 // Discord green
		payout := common.CalculateParlayPayout(parlay.Amount, parlay.TotalOdds)
		description.WriteString(fmt.Sprintf("<@%s> Your parlay has been **won**!\n\n", user.DiscordID))
		description.WriteString(fmt.Sprintf("**Amount Wagered:** %d points\n", parlay.Amount))
		description.WriteString(fmt.Sprintf("**Combined Odds:** %.2fx\n", parlay.TotalOdds))
		description.WriteString(fmt.Sprintf("**Payout:** %.1f points\n", payout))
	} else {
		title = "💔 Parlay Lost"
		color = 0xED4245 // Discord red
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
		// Log error but don't fail - notification is not critical
		fmt.Printf("Error sending parlay resolution notification: %v\n", err)
	}
}

// MyParlays shows the user's active parlays
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
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You have no active parlays.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	var response strings.Builder
	response.WriteString(fmt.Sprintf("You have %d active parlay(s):\n\n", len(parlays)))

	for parlayIdx, parlay := range parlays {
		response.WriteString(fmt.Sprintf("**Parlay #%d** (Amount: %d, Odds: %.2fx)\n", parlayIdx+1, parlay.Amount, parlay.TotalOdds))

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

			response.WriteString(fmt.Sprintf("  %d. %s: **%s** - %s\n", entryIdx+1, entry.Bet.Description, optionName, status))
		}

		potentialPayout := common.CalculateParlayPayout(parlay.Amount, parlay.TotalOdds)
		response.WriteString(fmt.Sprintf("  Potential Payout: %.1f points\n\n", potentialPayout))
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response.String(),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
	}
}
