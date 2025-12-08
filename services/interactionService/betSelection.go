package interactionService

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/services/betService"
	"perfectOddsBot/services/common"
	"strconv"
	"strings"
)

func HandleCBBGamePagination(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	if i.Type != discordgo.InteractionMessageComponent {
		return nil
	}

	// Extract session ID and page number from custom ID
	// Format: create_cbb_bet_previous_page_{page}_{sessionID} or create_cbb_bet_next_page_{page}_{sessionID}
	var sessionID string
	var currentPage int
	var err error

	if strings.HasPrefix(customID, "create_cbb_bet_previous_page_") {
		// Remove prefix to get "{page}_{sessionID}"
		rest := strings.TrimPrefix(customID, "create_cbb_bet_previous_page_")
		parts := strings.SplitN(rest, "_", 2)
		if len(parts) == 2 {
			currentPage, err = strconv.Atoi(parts[0])
			if err != nil {
				return fmt.Errorf("invalid page number in custom ID: %w", err)
			}
			sessionID = parts[1]
			currentPage--
		} else {
			// Fallback for old format without session ID
			currentPage, _ = strconv.Atoi(rest)
			currentPage--
		}
	} else if strings.HasPrefix(customID, "create_cbb_bet_next_page_") {
		// Remove prefix to get "{page}_{sessionID}"
		rest := strings.TrimPrefix(customID, "create_cbb_bet_next_page_")
		parts := strings.SplitN(rest, "_", 2)
		if len(parts) == 2 {
			currentPage, err = strconv.Atoi(parts[0])
			if err != nil {
				return fmt.Errorf("invalid page number in custom ID: %w", err)
			}
			sessionID = parts[1]
			currentPage++
		} else {
			// Fallback for old format without session ID
			currentPage, _ = strconv.Atoi(rest)
			currentPage++
		}
	}

	if sessionID == "" {
		return fmt.Errorf("session ID not found in custom ID")
	}

	// Retrieve paginated options for this session
	paginatedOptions, exists := betService.GetCBBPaginatedOptions(sessionID)
	if !exists {
		return fmt.Errorf("paginated options not found for session %s", sessionID)
	}

	if currentPage < 0 {
		currentPage = 0
	}
	if currentPage >= len(paginatedOptions) {
		currentPage = len(paginatedOptions) - 1
	}

	minValues := 1
	content := fmt.Sprintf("Select a game (Page %d/%d):", currentPage+1, len(paginatedOptions))
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: content,
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
							Disabled: currentPage == 0,
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
		return err
	}

	return nil
}

func HandleCFBGamePagination(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	if i.Type != discordgo.InteractionMessageComponent {
		return nil
	}

	// Extract session ID and page number from custom ID
	// Format: create_cfb_bet_previous_page_{page}_{sessionID} or create_cfb_bet_next_page_{page}_{sessionID}
	var sessionID string
	var currentPage int
	var err error

	if strings.HasPrefix(customID, "create_cfb_bet_previous_page_") {
		// Remove prefix to get "{page}_{sessionID}"
		rest := strings.TrimPrefix(customID, "create_cfb_bet_previous_page_")
		parts := strings.SplitN(rest, "_", 2)
		if len(parts) == 2 {
			currentPage, err = strconv.Atoi(parts[0])
			if err != nil {
				return fmt.Errorf("invalid page number in custom ID: %w", err)
			}
			sessionID = parts[1]
			currentPage--
		} else {
			// Fallback for old format without session ID
			currentPage, _ = strconv.Atoi(rest)
			currentPage--
		}
	} else if strings.HasPrefix(customID, "create_cfb_bet_next_page_") {
		// Remove prefix to get "{page}_{sessionID}"
		rest := strings.TrimPrefix(customID, "create_cfb_bet_next_page_")
		parts := strings.SplitN(rest, "_", 2)
		if len(parts) == 2 {
			currentPage, err = strconv.Atoi(parts[0])
			if err != nil {
				return fmt.Errorf("invalid page number in custom ID: %w", err)
			}
			sessionID = parts[1]
			currentPage++
		} else {
			// Fallback for old format without session ID
			currentPage, _ = strconv.Atoi(rest)
			currentPage++
		}
	}

	if sessionID == "" {
		return fmt.Errorf("session ID not found in custom ID")
	}

	// Retrieve paginated options for this session
	paginatedOptions, exists := betService.GetCFBPaginatedOptions(sessionID)
	if !exists {
		return fmt.Errorf("paginated options not found for session %s", sessionID)
	}

	if currentPage < 0 {
		currentPage = 0
	}
	if currentPage >= len(paginatedOptions) {
		currentPage = len(paginatedOptions) - 1
	}

	minValues := 1
	content := fmt.Sprintf("Select a game (Page %d/%d):", currentPage+1, len(paginatedOptions))
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: content,
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
							Disabled: currentPage == 0,
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
		return err
	}

	return nil
}

func HandleCBBGameSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	selectedGameID := i.MessageComponentData().Values[0]

	// Extract session ID from custom ID if present (format: create_cbb_bet_submit_{sessionID})
	// Clean up session data after bet is submitted
	if strings.HasPrefix(customID, "create_cbb_bet_submit_") {
		sessionID := strings.TrimPrefix(customID, "create_cbb_bet_submit_")
		betService.CleanupCBBPaginatedOptions(sessionID)
	}

	gameIDInt, err := strconv.Atoi(selectedGameID)
	if err != nil {
		common.SendError(s, i, err, db)
		return err
	}

	err = betService.CreateCBBBetFromGameID(s, i, db, gameIDInt)
	if err != nil {
		common.SendError(s, i, err, db)
		return err
	}

	return nil
}

func HandleCFBGameSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	selectedGameID := i.MessageComponentData().Values[0]

	// Extract session ID from custom ID if present (format: create_cfb_bet_submit_{sessionID})
	// Clean up session data after bet is submitted
	if strings.HasPrefix(customID, "create_cfb_bet_submit_") {
		sessionID := strings.TrimPrefix(customID, "create_cfb_bet_submit_")
		betService.CleanupCFBPaginatedOptions(sessionID)
	}

	gameIDInt, err := strconv.Atoi(selectedGameID)
	if err != nil {
		common.SendError(s, i, err, db)
		return err
	}

	err = betService.CreateCFBBetFromGameID(s, i, db, gameIDInt)
	if err != nil {
		common.SendError(s, i, err, db)
		return err
	}

	return nil
}

func HandleCBBGameCancel(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	// Extract session ID from custom ID (format: create_cbb_bet_cancel_{sessionID})
	sessionID := strings.TrimPrefix(customID, "create_cbb_bet_cancel_")
	
	// Clean up session data
	betService.CleanupCBBPaginatedOptions(sessionID)
	
	// Update the message to show cancellation
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ Bet creation cancelled.",
			Components: []discordgo.MessageComponent{},
		},
	})
	if err != nil {
		return fmt.Errorf("error responding to cancel: %w", err)
	}
	
	return nil
}

func HandleCFBGameCancel(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	// Extract session ID from custom ID (format: create_cfb_bet_cancel_{sessionID})
	sessionID := strings.TrimPrefix(customID, "create_cfb_bet_cancel_")
	
	// Clean up session data
	betService.CleanupCFBPaginatedOptions(sessionID)
	
	// Update the message to show cancellation
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: "❌ Bet creation cancelled.",
			Components: []discordgo.MessageComponent{},
		},
	})
	if err != nil {
		return fmt.Errorf("error responding to cancel: %w", err)
	}
	
	return nil
}

