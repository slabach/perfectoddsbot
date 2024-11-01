package interactionService

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
)

func PlaceBet(s *discordgo.Session, i *discordgo.InteractionCreate, customID string) error {
	// Handle placing a bet
	var betID uint
	var option string
	_, err := fmt.Sscanf(customID, "bet_%d_%s", &betID, &option)
	if err != nil {
		return err
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:    "Enter Bet Amount",
			CustomID: fmt.Sprintf("submit_bet_%d_%s", betID, option),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "bet_amount",
							Label:       "Bet Amount",
							Style:       discordgo.TextInputShort,
							Placeholder: "Enter amount",
							Required:    true,
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
