package cards

import "perfectOddsBot/models"

func registerEpicCards(deck *[]models.Card) {
	epicCards := []models.Card{
		// {
		// 	ID:          46,
		// 	Name:        "The Nuke",
		// 	Description: "Everyone (including you) loses 10% of their points to the Pool.",
		// 	Rarity:      "Legendary",
		// 	Weight:      W_Legendary,
		// 	Handler:     handleNuke,
		// },
		// {
		// 	ID:          47,
		// 	Name:        "Divine Intervention",
		// 	Description: "Your points balance is set to exactly the average of all players.",
		// 	Rarity:      "Legendary",
		// 	Weight:      W_Legendary,
		// 	Handler:     handleDivineIntervention,
		// },
		// {
		// 	ID:          48,
		// 	Name:        "Hostile Takeover",
		// 	Description: "Swap your point balance with a user of your choice (Max swap 2,000 points).",
		// 	Rarity:      "Legendary",
		// 	Weight:      W_Legendary,
		// 	Handler:     handleHostileTakeover,
		// },
		// {
		// 	ID:          49,
		// 	Name:        "The Whale",
		// 	Description: "Gain 1000 points immediately.",
		// 	Rarity:      "Legendary",
		// 	Weight:      W_Legendary,
		// 	Handler:     handleWhale,
		// },
		// {
		// 	ID:          50,
		// 	Name:        "Market Crash",
		// 	Description: "All active bets currently placed are cancelled; all money bet on them goes to the Pool.",
		// 	Rarity:      "Legendary",
		// 	Weight:      W_Legendary,
		// 	Handler:     handleMarketCrash,
		// },
		// {
		// 	ID:                   53,
		// 	Name:                 "Guillotine",
		// 	Description:          "You lose 15% of your points.",
		// 	Rarity:               "Legendary",
		// 	Weight:               W_Legendary,
		// 	Handler:              handleJackpot,
		// 	RoyaltyDiscordUserID: &[]string{"130863485969104896"}[0],
		// },
	}

	// Add to deck
	*deck = append(*deck, epicCards...)
}
