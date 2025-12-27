package interactionService

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"strconv"
)

func SubmitBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	var betID uint
	var option string
	var optionVal int

	guild, err := guildService.GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		return err
	}

	_, err = fmt.Sscanf(customID, "submit_bet_%d_%s", &betID, &option)
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
	result := db.FirstOrCreate(&user, models.User{DiscordID: userID, GuildID: guildID})
	if result.RowsAffected == 1 {
		user.Points = guild.StartingPoints
	}
	
	// Update username from interaction member (always update to keep it current)
	username := common.GetUsernameFromUser(i.Member.User)
	common.UpdateUserUsername(db, &user, username)
	
	if result.RowsAffected == 1 {
		db.Save(&user)
	}

	if user.Points < float64(amount) {
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

	user.Points -= float64(amount)
	db.Save(&user)

	optionName := bet.Option1
	if optionVal == 2 {
		optionName = bet.Option2
	}

	// Calculate potential payout
	potentialPayout := common.CalculatePayout(amount, optionVal, bet)

	// Create success embed
	embed := &discordgo.MessageEmbed{
		Title:       "✅ Bet Placed Successfully",
		Description: fmt.Sprintf("You've placed **%d** points on **%s**", amount, optionName),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Remaining Points",
				Value:  fmt.Sprintf("%.1f", user.Points),
				Inline: true,
			},
			{
				Name:   "Potential Payout",
				Value:  fmt.Sprintf("%.1f", potentialPayout),
				Inline: true,
			},
		},
		Color: 0x00ff00,
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return errors.New(fmt.Sprintf("Error sending message: %v", err))
	}
	return nil
}
