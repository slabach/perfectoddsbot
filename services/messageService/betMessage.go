package messageService

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"perfectOddsBot/services/common"
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
