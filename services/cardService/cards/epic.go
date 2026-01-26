package cards

import "perfectOddsBot/models"

func registerEpicCards(deck *[]models.Card) {
	epicCards := []models.Card{
		{
			ID:                   38,
			Code:                 "BLU",
			Name:                 "The Blue Shell",
			Description:          "The player in 1st place loses 10%.",
			Handler:              handleBlueShell,
			RoyaltyDiscordUserID: &[]string{"698712210515558432"}[0],
		},
		{
			ID:          46,
			Code:        "NUK",
			Name:        "The Nuke",
			Description: "Everyone (including you and the pool) loses 25% of their points.",
			Handler:     handleNuke,
		},
		{
			ID:          47,
			Code:        "DIV",
			Name:        "Divine Intervention",
			Description: "Your points balance is set to exactly the average of all players.",
			Handler:     handleDivineIntervention,
		},
		{
			ID:          48,
			Code:        "HOS",
			Name:        "Hostile Takeover",
			Description: "Swap your point balance with a user of your choice (Max swap 500 points (players must be within 500 points of you)).",
			Handler:     handleHostileTakeover,
		},
		{
			ID:          49,
			Code:        "WHA",
			Name:        "The Whale",
			Description: "Gain 750 points immediately.",
			Handler:     handleWhale,
		},
		{
			ID:                   53,
			Code:                 "GUI",
			Name:                 "Guillotine",
			Description:          "You lose 15% of your points.",
			Handler:              handleGuillotine,
			RoyaltyDiscordUserID: &[]string{"130863485969104896"}[0],
		},
		{
			ID:                   62,
			Code:                 "EMO",
			Name:                 "Emotional Hedge",
			Description:          "Your next bet on your server's subscribed team, if they lose straight up, you get 50% of your bet refunded.",
			Handler:              handleEmotionalHedge,
			RoyaltyDiscordUserID: &[]string{"972670149247258634"}[0],
			AddToInventory:       true,
			RequiredSubscription: true,
		},
		{
			ID:                   27,
			Code:                 "DDN",
			Name:                 "Double Down",
			Description:          "The payout of your next winning bet is increased by 2x",
			Handler:              handleDoubleDown,
			AddToInventory:       true,
			RoyaltyDiscordUserID: &[]string{"130863485969104896"}[0],
		},
		{
			ID:                   70,
			Code:                 "STP",
			Name:                 "STOP THE STEAL",
			Description:          "**PLAYABLE CARD** Play this card at any point to choose any active bet you have open and cancel it.",
			Handler:              handleStopTheSteal,
			RoyaltyDiscordUserID: &[]string{"195444122578845696"}[0],
			UserPlayable:         true,
		},
		{
			ID:                   66,
			Code:                 "SNP",
			Name:                 "Snip Snap Snip Snap",
			Description:          "If you have any active bets, one of them randomly gets their option reversed.",
			Handler:              handleSnipSnap,
			RoyaltyDiscordUserID: &[]string{"447827835797766144"}[0],
		},
		{
			ID:          71,
			Code:        "RTH",
			Name:        "Robbing the Hood",
			Description: "Steal 10% of the poorest player's points and give it to yourself.",
			Handler:     handleRobbingTheHood,
		},
		{
			ID:          74,
			Code:        "LEH",
			Name:        "Lehman Brothers Insider",
			Description: "You penetrated the banks internal systems and brought them down from the inside. The pool loses 20% of its total points.",
			Handler:     handleLehmanBrothersInsider,
		},
		// {
		// 	ID:                   76,
		// 	Name:                 "Hot Hand",
		// 	Description:          "For every consecutive bet won with this card, your payout multiplier increases by +0.5x (1.5x, 2.0x, 2.5x...). A loss resets the multiplier to normal and the card is removed from your inventory.",
		// 	Handler:              handleHotHand,
		// 	RoyaltyDiscordUserID: &[]string{"698712210515558432"}[0],
		// },
		{
			ID:             78,
			Code:           "LCH",
			Name:           "Leech",
			Description:    "You attach yourself to the richest player in the pool and siphon 1% of their points every hour for the next 12 hours.",
			Handler:        handleLeech,
			AddToInventory: true,
		},

		{
			ID:                   86,
			Code:                 "POB",
			Name:                 "Pool Boy",
			Description:          "**PLAYABLE CARD** Play this card at any point to clean the algae from the pool and stop the drain.",
			Handler:              handlePoolBoy,
			UserPlayable:         true,
			AddToInventory:       true,
			RoyaltyDiscordUserID: &[]string{"313553928115716097"}[0],
		},
		{
			ID:                   87,
			Code:                 "BHO",
			Name:                 "Black Hole",
			Description:          "25% of the pool is evenly distruted evenly among the bottom 5 players.",
			Handler:              handleBlackHole,
			RoyaltyDiscordUserID: &[]string{"130863485969104896"}[0],
		},
	}

	*deck = append(*deck, epicCards...)
}
