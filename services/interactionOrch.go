package services

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"log"
	"predictionOddsBot/models"
	"strconv"
	"strings"
)

func HandleComponentInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	if strings.HasPrefix(customID, "resolve_bet_") {
		betID, err := strconv.Atoi(strings.TrimPrefix(customID, "resolve_bet_"))
		if err != nil {
			log.Printf("Error parsing bet ID: %v", err)
			return
		}

		if !IsAdmin(s, i) {
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "You are not authorized to use this command.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				return
			}
			return
		}

		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				Title:    "Resolve Bet",
				CustomID: fmt.Sprintf("resolve_bet_confirm_%d", betID),
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    "winning_option",
								Label:       "Enter Winning Option (1 or 2)",
								Style:       discordgo.TextInputShort,
								Placeholder: "1 or 2",
								Required:    true,
							},
						},
					},
				},
			},
		})
		if err != nil {
			log.Printf("Error presenting modal: %v", err)
			return
		}
	}
}

func HandleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	customID := i.ModalSubmitData().CustomID

	if strings.HasPrefix(customID, "resolve_bet_confirm_") {
		betIDStr := strings.TrimPrefix(customID, "resolve_bet_confirm_")
		betID, err := strconv.Atoi(betIDStr)
		if err != nil {
			log.Printf("Error parsing bet ID: %v", err)
			return
		}

		selectedOption := i.ModalSubmitData().Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
		winningOption, err := strconv.Atoi(selectedOption)
		if err != nil {
			log.Printf("Error parsing selected option: %v", err)
			return
		}

		ResolveBetByID(s, i, betID, winningOption, db)
		return
	}

	var betID uint
	var option int

	_, err := fmt.Sscanf(customID, "placebet_%d_%d", &betID, &option)
	if err != nil {
		log.Printf("Error parsing modal customID for placing a bet: %v", err)
		return
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
			return
		}
		return
	}

	userID := i.Member.User.ID
	guildID := i.GuildID

	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: userID, GuildID: guildID})
	if result.RowsAffected == 1 {
		user.Points = 1000
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
			return
		}
		return
	}

	var bet models.Bet
	result = db.First(&bet, "id = ? AND guild_id = ? AND active = ?", betID, guildID, true)
	if result.Error != nil || bet.ID == 0 {
		response := "This bet is no longer active or does not exist."
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return
		}
		return
	}

	betEntry := models.BetEntry{
		UserID: user.ID,
		BetID:  betID,
		Option: option,
		Amount: amount,
	}
	db.Create(&betEntry)

	user.Points -= amount
	db.Save(&user)

	optionName := bet.Option1
	if option == 2 {
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
		return
	}
}
