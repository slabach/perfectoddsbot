package interactionService

import (
	"fmt"
	"math"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func PlaceBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
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

	username := common.GetUsernameFromUser(i.Member.User)
	common.UpdateUserUsername(db, &user, username)

	if user.BetLockoutUntil != nil && user.BetLockoutUntil.After(time.Now()) {
		timeLeft := time.Until(*user.BetLockoutUntil).Round(time.Minute)
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("❄️ You are frozen from betting! You can bet again in %s.", timeLeft),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
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
