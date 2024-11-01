package interactionService

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/messageService"
	"strconv"
	"strings"
)

func LockBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	// Handle locking a bet
	betID, err := strconv.Atoi(strings.TrimPrefix(customID, "lock_bet_"))
	if err != nil {
		return errors.New(fmt.Sprintf("Error parsing bet ID: %v", err))
	}

	if !common.IsAdmin(s, i) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You are not authorized to use this command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return errors.New(fmt.Sprintf("Error sending unauthorized message: %v", err))
		}
		return nil
	}

	var bet models.Bet
	result := db.First(&bet, "id = ?", betID)
	if result.Error != nil || bet.ID == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Bet not found.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return errors.New(fmt.Sprintf("Error sending bet not found message: %v", err))
		}
		return nil
	}

	bet.Active = false
	db.Save(&bet)

	_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:      i.Message.ID,
		Channel: i.ChannelID,
		Components: &[]discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{messageService.GetResolveButton(bet.ID)},
			},
		},
	})
	if err != nil {
		return errors.New(fmt.Sprintf("Error removing buttons from the message: %v", err))
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Bet '%s' has been locked and is no longer accepting new bets.", bet.Description),
		},
	})
	if err != nil {
		return errors.New(fmt.Sprintf("Error sending bet locked message: %v", err))
	}

	return nil
}
