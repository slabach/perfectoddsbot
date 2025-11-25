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

	currentPage := 0

	if strings.HasPrefix(customID, "create_cbb_bet_previous_page") {
		pageStr := strings.TrimPrefix(customID, "create_cbb_bet_previous_page_")
		currentPage, _ = strconv.Atoi(pageStr)
		currentPage--
	}
	if strings.HasPrefix(customID, "create_cbb_bet_next_page") {
		pageStr := strings.TrimPrefix(customID, "create_cbb_bet_next_page_")
		currentPage, _ = strconv.Atoi(pageStr)
		currentPage++
	}

	if currentPage < 0 {
		currentPage = 0
	}
	if currentPage >= len(betService.CBBPaginatedOptions) {
		currentPage = len(betService.CBBPaginatedOptions) - 1
	}

	minValues := 1
	content := fmt.Sprintf("Select a game (Page %d/%d):", currentPage+1, len(betService.CBBPaginatedOptions))
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    "create_cbb_bet_submit",
							Placeholder: "Select a game",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     betService.CBBPaginatedOptions[currentPage],
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Previous",
							CustomID: fmt.Sprintf("create_cbb_bet_previous_page_%d", currentPage),
							Style:    discordgo.PrimaryButton,
							Disabled: currentPage == 0,
						},
						discordgo.Button{
							Label:    "Next",
							CustomID: fmt.Sprintf("create_cbb_bet_next_page_%d", currentPage),
							Style:    discordgo.PrimaryButton,
							Disabled: currentPage == len(betService.CBBPaginatedOptions)-1,
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

	currentPage := 0

	if strings.HasPrefix(customID, "create_cfb_bet_previous_page") {
		pageStr := strings.TrimPrefix(customID, "create_cfb_bet_previous_page_")
		currentPage, _ = strconv.Atoi(pageStr)
		currentPage--
	}
	if strings.HasPrefix(customID, "create_cfb_bet_next_page") {
		pageStr := strings.TrimPrefix(customID, "create_cfb_bet_next_page_")
		currentPage, _ = strconv.Atoi(pageStr)
		currentPage++
	}

	if currentPage < 0 {
		currentPage = 0
	}
	if currentPage >= len(betService.CFBPaginatedOptions) {
		currentPage = len(betService.CFBPaginatedOptions) - 1
	}

	minValues := 1
	content := fmt.Sprintf("Select a game (Page %d/%d):", currentPage+1, len(betService.CFBPaginatedOptions))
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    "create_cfb_bet_submit",
							Placeholder: "Select a game",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     betService.CFBPaginatedOptions[currentPage],
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Previous",
							CustomID: fmt.Sprintf("create_cfb_bet_previous_page_%d", currentPage),
							Style:    discordgo.PrimaryButton,
							Disabled: currentPage == 0,
						},
						discordgo.Button{
							Label:    "Next",
							CustomID: fmt.Sprintf("create_cfb_bet_next_page_%d", currentPage),
							Style:    discordgo.PrimaryButton,
							Disabled: currentPage == len(betService.CFBPaginatedOptions)-1,
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

