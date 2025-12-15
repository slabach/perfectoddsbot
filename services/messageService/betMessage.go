package messageService

import (
	"fmt"
	"perfectOddsBot/services/common"

	"github.com/bwmarrin/discordgo"
)

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

// BuildBetResolutionEmbed creates a consistent embed for bet resolution messages.
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
		Color:       0x57F287, // green-ish
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
