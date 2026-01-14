package cards

import "perfectOddsBot/models"

func registerEpicCards(deck *[]models.Card) {
	epicCards := []models.Card{
		// {
		// 	ID:                   38,
		// 	Name:                 "The Blue Shell",
		// 	Description:          "The player in 1st place loses 500 points to the Pool.",
		// 	Rarity:               "Rare",
		// 	Weight:               W_Rare,
		// 	Handler:              handleBlueShell,
		// 	RoyaltyDiscordUserID: &[]string{"698712210515558432"}[0],
		// },
		// {
		// 	ID:          46,
		// 	Name:        "The Nuke",
		// 	Description: "Everyone (including you) loses 10% of their points to the Pool.",
		// 	Rarity:      "Legendary",
		// 	Weight:      W_Epic,
		// 	Handler:     handleNuke,
		// },
		// {
		// 	ID:          47,
		// 	Name:        "Divine Intervention",
		// 	Description: "Your points balance is set to exactly the average of all players.",
		// 	Rarity:      "Legendary",
		// 	Weight:      W_Epic,
		// 	Handler:     handleDivineIntervention,
		// },
		// {
		// 	ID:          48,
		// 	Name:        "Hostile Takeover",
		// 	Description: "Swap your point balance with a user of your choice (Max swap 2,000 points).",
		// 	Rarity:      "Legendary",
		// 	Weight:      W_Epic,
		// 	Handler:     handleHostileTakeover,
		// },
		// {
		// 	ID:          49,
		// 	Name:        "The Whale",
		// 	Description: "Gain 1000 points immediately.",
		// 	Rarity:      "Legendary",
		// 	Weight:      W_Epic,
		// 	Handler:     handleWhale,
		// },
		// {
		// 	ID:          50,
		// 	Name:        "Market Crash",
		// 	Description: "All active bets currently placed are cancelled; all money bet on them goes to the Pool.",
		// 	Rarity:      "Legendary",
		// 	Weight:      W_Epic,
		// 	Handler:     handleMarketCrash,
		// },
		// {
		// 	ID:                   53,
		// 	Name:                 "Guillotine",
		// 	Description:          "You lose 15% of your points.",
		// 	Rarity:               "Legendary",
		// 	Weight:               W_Epic,
		// 	Handler:              handleJackpot,
		// 	RoyaltyDiscordUserID: &[]string{"130863485969104896"}[0],
		// },

		{
			ID:                   62,
			Name:                 "Emotional Hedge",
			Description:          "Your next bet on your server's subscribed team, if they lose straight up, you get 50% of your bet refunded.",
			Rarity:               "Legendary",
			Weight:               W_Epic,
			Handler:              handleEmotionalHedge,
			RoyaltyDiscordUserID: &[]string{"972670149247258634"}[0],
			AddToInventory:       true,
			RequiredSubscription: true,
		},
		// {
		// 	ID:          68,
		// 	Name:        "The Gambler",
		// 	Description: "**CHOICE CARD** You choose one of the following options:",
		// 	Options: []models.CardOption{
		// 		{
		// 			ID:          1,
		// 			Name:        "Yes",
		// 			Description: "50/50 chance to win 2x your bet, or double your loss.",
		// 		},
		// 		{
		// 			ID:          2,
		// 			Name:        "No",
		// 			Description: "Nothing happens.",
		// 		},
		// 	},
		// 	Rarity:               "Epic",
		// 	Weight:               W_Epic,
		// 	Handler:              handleGambler,
		// 	RoyaltyDiscordUserID: &[]string{"447827835797766144"}[0],
		// },
	}

	// Add to deck
	*deck = append(*deck, epicCards...)
}
