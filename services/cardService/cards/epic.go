package cards

import "perfectOddsBot/models"

func registerEpicCards(deck *[]models.Card) {
	epicCards := []models.Card{
		{
			ID:                   38,
			Name:                 "The Blue Shell",
			Description:          "The player in 1st place loses 500 points to the Pool.",
			Rarity:               "Epic",
			Weight:               W_Epic,
			Handler:              handleBlueShell,
			RoyaltyDiscordUserID: &[]string{"698712210515558432"}[0],
		},
		{
			ID:          46,
			Name:        "The Nuke",
			Description: "Everyone (including you) loses 10% of their points to the Pool.",
			Rarity:      "Epic",
			Weight:      W_Epic,
			Handler:     handleNuke,
		},
		{
			ID:          47,
			Name:        "Divine Intervention",
			Description: "Your points balance is set to exactly the average of all players.",
			Rarity:      "Epic",
			Weight:      W_Epic,
			Handler:     handleDivineIntervention,
		},
		{
			ID:          48,
			Name:        "Hostile Takeover",
			Description: "Swap your point balance with a user of your choice (Max swap 500 points (players must be within 500 points of you)).",
			Rarity:      "Epic",
			Weight:      W_Epic,
			Handler:     handleHostileTakeover,
		},
		{
			ID:          49,
			Name:        "The Whale",
			Description: "Gain 750 points immediately.",
			Rarity:      "Epic",
			Weight:      W_Epic,
			Handler:     handleWhale,
		},
		{
			ID:                   53,
			Name:                 "Guillotine",
			Description:          "You lose 15% of your points.",
			Rarity:               "Epic",
			Weight:               W_Epic,
			Handler:              handleGuillotine,
			RoyaltyDiscordUserID: &[]string{"130863485969104896"}[0],
		},
		{
			ID:                   62,
			Name:                 "Emotional Hedge",
			Description:          "Your next bet on your server's subscribed team, if they lose straight up, you get 50% of your bet refunded.",
			Rarity:               "Epic",
			Weight:               W_Epic,
			Handler:              handleEmotionalHedge,
			RoyaltyDiscordUserID: &[]string{"972670149247258634"}[0],
			AddToInventory:       true,
			RequiredSubscription: true,
		},
		{
			ID:          72,
			Name:        "The Gambler",
			Description: "**CHOICE CARD** You choose one of the following options:",
			Options: []models.CardOption{
				{
					ID:          1,
					Name:        "Yes",
					Description: "50/50 chance to win 2x your next bet that gets resolved, or double your loss.",
				},
				{
					ID:          2,
					Name:        "No",
					Description: "Nothing happens.",
				},
			},
			Rarity:               "Epic",
			Weight:               W_Epic,
			Handler:              handleGambler,
			RoyaltyDiscordUserID: &[]string{"447827835797766144"}[0],
			AddToInventory:       true,
		},
		{
			ID:                   70,
			Name:                 "STOP THE STEAL",
			Description:          "**PLAYABLE CARD** Play this card at any point to choose any active bet you have open and cancel it.",
			Rarity:               "Epic",
			Weight:               W_Epic,
			Handler:              handleStopTheSteal,
			RoyaltyDiscordUserID: &[]string{"195444122578845696"}[0],
			UserPlayable:         true,
		},
		{
			ID:                   66,
			Name:                 "Snip Snap Snip Snap",
			Description:          "If you have any active bets, one of them randomly gets their option reversed.",
			Rarity:               "Epic",
			Weight:               W_Epic,
			Handler:              handleSnipSnap,
			RoyaltyDiscordUserID: &[]string{"447827835797766144"}[0],
		},
		{
			ID:          71,
			Name:        "Robbing the Hood",
			Description: "Steal 50% of the poorest player's points and give it to yourself.",
			Rarity:      "Epic",
			Weight:      W_Epic,
			Handler:     handleRobbingTheHood,
		},
	}

	// Add to deck
	*deck = append(*deck, epicCards...)
}
