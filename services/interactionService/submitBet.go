package interactionService

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"log"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"strconv"
)

func SubmitBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	var betID uint
	var option string
	var optionVal int

	log.Printf(customID)
	_, err := fmt.Sscanf(customID, "submit_bet_%d_%s", &betID, &option)
	if err != nil {
		return errors.New(fmt.Sprintf("Error parsing modal customID for placing a bet: %v", err))
	} else {
		_, err = fmt.Sscanf(option, "option%d", &optionVal)
	}

	amountStr := i.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	amount, err := strconv.Atoi(amountStr)
	if err != nil || amount <= 0 {
		response := "Invalid bet amount. Please enter a positive number."
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return errors.New(fmt.Sprintf("Error placing bet: %v", err))
		}
		return nil
	}

	userID := i.Member.User.ID
	guildID := i.GuildID

	var user models.User
	username := common.GetUsername(s, guildID, userID)
	result := db.FirstOrCreate(&user, models.User{DiscordID: userID, GuildID: guildID})
	if result.RowsAffected == 1 {
		user.Points = 1000
		if user.Username == nil {
			user.Username = &username
		}
		db.Save(&user)
	}

	if user.Points < amount {
		response := "You do not have enough points to place this bet."
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return errors.New(fmt.Sprintf("Error sending message: %v", err))
		}
		return nil
	}

	var bet models.Bet
	result = db.First(&bet, "id = ? AND guild_id = ? AND active = ?", betID, guildID, true)
	if result.Error != nil || bet.ID == 0 {
		response := "This bet is closed."
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return errors.New(fmt.Sprintf("Error sending message: %v", err))
		}
		return nil
	}

	betEntry := models.BetEntry{
		UserID: user.ID,
		BetID:  betID,
		Option: optionVal,
		Amount: amount,
	}
	if bet.Spread != nil {
		betEntry.Spread = bet.Spread
	}
	db.Create(&betEntry)

	user.Points -= amount
	db.Save(&user)

	optionName := bet.Option1
	if optionVal == 2 {
		optionName = bet.Option2
	}

	response := fmt.Sprintf("Successfully placed a bet of **%d** points on **%s**.", amount, optionName)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return errors.New(fmt.Sprintf("Error sending message: %v", err))
	}
	return nil
}
