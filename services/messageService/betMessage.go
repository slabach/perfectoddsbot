package messageService

import (
	"fmt"
	"log"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func CloseBetMessages(s *discordgo.Session, db *gorm.DB, bet models.Bet, title string) {
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: bet.Description,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  fmt.Sprintf("1️⃣ %s", bet.Option1),
				Value: fmt.Sprintf("Odds: %s", common.FormatOdds(float64(bet.Odds1))),
			},
			{
				Name:  fmt.Sprintf("2️⃣ %s", bet.Option2),
				Value: fmt.Sprintf("Odds: %s", common.FormatOdds(float64(bet.Odds2))),
			},
		},
		Color: 0x3498db,
	}

	if bet.MessageID != nil {
		_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID:         *bet.MessageID,
			Channel:    bet.ChannelID,
			Embeds:     &[]*discordgo.MessageEmbed{embed},
			Components: &[]discordgo.MessageComponent{},
		})
		if err != nil {
			log.Printf("Error closing Discord message for bet %d: %v", bet.ID, err)
		}
	}

	var secondaryMsgs []models.BetMessage
	if err := db.Where("active = 1 AND bet_id = ?", bet.ID).Find(&secondaryMsgs).Error; err != nil {
		log.Printf("Error finding secondary messages for bet %d: %v", bet.ID, err)
		return
	}

	for _, msg := range secondaryMsgs {
		msg.Active = false
		if err := db.Save(&msg).Error; err != nil {
			log.Printf("Error deactivating secondary message %d for bet %d: %v", msg.ID, bet.ID, err)
			continue
		}
		if msg.MessageID == nil {
			continue
		}
		_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID:         *msg.MessageID,
			Channel:    msg.ChannelID,
			Embeds:     &[]*discordgo.MessageEmbed{embed},
			Components: &[]discordgo.MessageComponent{},
		})
		if err != nil {
			log.Printf("Error closing secondary Discord message for bet %d: %v", bet.ID, err)
		}
	}
}

func GetAllButtonList(s *discordgo.Session, i *discordgo.InteractionCreate, opt1 string, opt2 string, betId uint) []discordgo.MessageComponent {
	var buttons []discordgo.MessageComponent

	betButtons := GetBetButtons(opt1, opt2, betId)

	for _, betButton := range betButtons {
		buttons = append(buttons, betButton)
	}

	if common.IsAdmin(s, i) {
		resolveBtn := GetResolveButton(betId)
		lockBtn := GetLockButton(betId)
		buttons = append(buttons, lockBtn)
		buttons = append(buttons, resolveBtn)
	}

	return buttons
}

func GetBetOnlyButtonsList(opt1 string, opt2 string, betId uint) []discordgo.MessageComponent {
	var buttons []discordgo.MessageComponent

	betButtons := GetBetButtons(opt1, opt2, betId)

	for _, betButton := range betButtons {
		buttons = append(buttons, betButton)
	}

	return buttons
}

func GetBetButtons(opt1 string, opt2 string, betId uint) []discordgo.Button {
	return []discordgo.Button{
		{
			Label:    opt1,
			Style:    discordgo.PrimaryButton,
			CustomID: fmt.Sprintf("bet_%d_option1", betId),
			Emoji: &discordgo.ComponentEmoji{
				Name: "🟡",
			},
		},
		{
			Label:    opt2,
			Style:    discordgo.SuccessButton,
			CustomID: fmt.Sprintf("bet_%d_option2", betId),
			Emoji: &discordgo.ComponentEmoji{
				Name: "🟡",
			},
		},
	}
}

func GetLockButton(betId uint) discordgo.Button {
	return discordgo.Button{
		Label:    "Close Betting",
		Style:    discordgo.DangerButton,
		CustomID: fmt.Sprintf("lock_bet_%d", betId),
		Emoji: &discordgo.ComponentEmoji{
			Name: "🔒",
		},
	}
}

func GetResolveButton(betId uint) discordgo.Button {
	return discordgo.Button{
		Label:    "Resolve Bet",
		Style:    discordgo.SecondaryButton,
		CustomID: fmt.Sprintf("resolve_bet_%d", betId),
		Emoji: &discordgo.ComponentEmoji{
			Name: "✅",
		},
	}
}

func BuildBetResolutionEmbed(betDescription string, subtitle string, totalPayout float64, winners string, losers string) *discordgo.MessageEmbed {
	if winners == "" {
		winners = "_No winners_"
	}
	if losers == "" {
		losers = "_No losers_"
	}

	return &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🏁 Bet Resolved: %s", betDescription),
		Description: subtitle,
		Color:       0x57F287,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Total Payout",
				Value:  fmt.Sprintf("**%.1f** points", totalPayout),
				Inline: true,
			},
			{
				Name:  "Winners",
				Value: winners,
			},
			{
				Name:  "Losers",
				Value: losers,
			},
		},
	}
}
