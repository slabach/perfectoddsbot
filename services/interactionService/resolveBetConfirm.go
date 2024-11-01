package interactionService

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/services/betService"
	"strconv"
	"strings"
)

func ResolveBetConfirm(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	betIDStr := strings.TrimPrefix(customID, "resolve_bet_confirm_")
	betID, err := strconv.Atoi(betIDStr)
	if err != nil {
		return errors.New(fmt.Sprintf("Error parsing bet ID: %v", err))
	}

	selectedOption := i.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	winningOption, err := strconv.Atoi(selectedOption)
	if err != nil {
		return errors.New(fmt.Sprintf("Error parsing selected option: %v", err))
	}

	betService.ResolveBetByID(s, i, betID, winningOption, db)

	_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         i.Message.ID,
		Channel:    i.ChannelID,
		Components: &[]discordgo.MessageComponent{},
	})
	if err != nil {
		return errors.New(fmt.Sprintf("Error removing buttons from the message: %v", err))
	}

	var secondaryMsgs []models.BetMessage
	secondaryResult := db.Where("active = 1 AND bet_id = ?", betID).Find(&secondaryMsgs)
	if secondaryResult.Error != nil {
		return errors.New(fmt.Sprintf("Error finding secondary messageService: %v", secondaryResult.Error))
	}
	if len(secondaryMsgs) > 0 {
		for _, msg := range secondaryMsgs {
			msg.Active = false
			db.Save(&msg)

			_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:         *msg.MessageID,
				Channel:    msg.ChannelID,
				Components: &[]discordgo.MessageComponent{},
			})
			if err != nil {
				continue
			}
		}
	}

	return nil
}
