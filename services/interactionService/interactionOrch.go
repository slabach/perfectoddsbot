package interactionService

import (
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/services/common"
	"strings"
)

func HandleComponentInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	customID := i.MessageComponentData().CustomID

	if strings.HasPrefix(customID, "bet_") {
		err := PlaceBet(s, i, db, customID)
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	if strings.HasPrefix(customID, "resolve_bet_") {
		err := ResolveBet(s, i, customID)
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	if strings.HasPrefix(customID, "lock_bet_") {
		err := LockBet(s, i, db, customID)
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	if strings.HasPrefix(customID, "subscribe_to_team_next_") || strings.HasPrefix(customID, "subscribe_to_team_previous_") {
		err := HandlePagination(s, i, db, customID)
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	if strings.HasPrefix(customID, "subscribe_to_team_submit") {
		err := HandleTeamSubscribeSubmit(s, i, db, customID)
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}
}

func HandleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	customID := i.ModalSubmitData().CustomID

	if strings.HasPrefix(customID, "resolve_bet_confirm_") {
		err := ResolveBetConfirm(s, i, db, customID)
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	if strings.HasPrefix(customID, "submit_bet_") {
		err := SubmitBet(s, i, db, customID)
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}
}
