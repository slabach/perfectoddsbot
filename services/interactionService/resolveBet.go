package interactionService

import (
	"errors"
	"fmt"
	"perfectOddsBot/services/common"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func ResolveBet(s *discordgo.Session, i *discordgo.InteractionCreate, customID string) error {
	betID, err := strconv.Atoi(strings.TrimPrefix(customID, "resolve_bet_"))
	if err != nil {
		return err
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
		return err
	}
	return nil
}
