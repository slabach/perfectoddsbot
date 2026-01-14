package cards

import "perfectOddsBot/models"

func registerRareCards(deck *[]models.Card) {
	rareCards := []models.Card{
		// {
		// 	ID:          39,
		// 	Name:        "The Red Shell",
		// 	Description: "Choose a player. They lose 10% of their points to you.",
		// 	Rarity:      "Rare",
		// 	Weight:      W_Rare,
		// 	Handler:     handleRedShell,
		// },
		// {
		// 	ID:          40,
		// 	Name:        "Uno Reverse",
		// 	Description: "Select an active bet you placed. If it loses, you win (and vice versa).",
		// 	Rarity:      "Rare",
		// 	Weight:      W_Rare,
		// 	Handler:     handleUnoReverse,
		// },
		// {
		// 	ID:          41,
		// 	Name:        "Socialism",
		// 	Description: "Take 5% from the Top 3 players and distribute it evenly among the Bottom 3.",
		// 	Rarity:      "Rare",
		// 	Weight:      W_Rare,
		// 	Handler:     handleSocialism,
		// },
		// {
		// 	ID:          42,
		// 	Name:        "Robin Hood",
		// 	Description: "Steal 300 points from the richest player; keep 100, give 200 to the poorest player.",
		// 	Rarity:      "Rare",
		// 	Weight:      W_Rare,
		// 	Handler:     handleRobinHood,
		// },
		// {
		// 	ID:             44,
		// 	Name:           "The Vampire",
		// 	Description:    "For the next 24 hours, earn 1% of every bet won by other players.",
		// 	Rarity:         "Rare",
		// 	Weight:         W_Rare,
		// 	Handler:        handleVampire,
		// 	AddToInventory: true,
		// },
		// {
		// 	ID:          45,
		// 	Name:        "Chaos Dunk",
		// 	Description: "Randomize the points of the middle 3 players on the leaderboard.",
		// 	Rarity:      "Rare",
		// 	Weight:      W_Rare,
		// 	Handler:     handleChaosDunk,
		// },
		// {
		// 	ID:                   64,
		// 	Name:                 "Red Shells",
		// 	Description:          "The 3 people directly in front of you on the leaderboard randomly lose between 10-25 points.",
		// 	Rarity:               "Rare",
		// 	Weight:               W_Rare,
		// 	Handler:              handleRedShells,
		// 	RoyaltyDiscordUserID: &[]string{"447827835797766144"}[0],
		// },
		{
			ID:                   67,
			Name:                 "Factory Reset",
			Description:          "If you have less than 1000 points, you are reset to 1000 points.",
			Rarity:               "Rare",
			Weight:               W_Rare,
			Handler:              handleFactoryReset,
			RoyaltyDiscordUserID: &[]string{"447827835797766144"}[0],
		},
		// {
		// 	ID:                   68,
		// 	Name:                 "Anti-Anti-Bet",
		// 	Description:          "Choose a player. You place a new bet that that user will lose their next bet they place.",
		// 	Rarity:               "Rare",
		// 	Weight:               W_Rare,
		// 	Handler:              handleAntiAntiBet,
		// 	RoyaltyDiscordUserID: &[]string{"238274131722764288"}[0],
		// },
	}

	// Add to deck
	*deck = append(*deck, rareCards...)
}
