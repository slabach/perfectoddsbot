package interactionService

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"math"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
)

func PlaceBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	// Handle placing a bet
	var betID uint
	var option string
	_, err := fmt.Sscanf(customID, "bet_%d_%s", &betID, &option)
	if err != nil {
		return err
	}

	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: i.Member.User.ID, GuildID: i.GuildID})
	if result.Error != nil {
		return result.Error
	}
	
	// Update username from interaction member
	username := common.GetUsernameFromUser(i.Member.User)
	common.UpdateUserUsername(db, &user, username)

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
	if err != nil {
		return err
	}
	return nil
}
