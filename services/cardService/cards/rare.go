package cards

import "perfectOddsBot/models"

func registerRareCards(deck *[]models.Card) {
	rareCards := []models.Card{
		{
			ID:          40,
			Name:        "Uno Reverse",
			Description: "Select an active bet you placed. If it loses, you win (and vice versa).",
			Rarity:      "Rare",
			Weight:      W_Rare,
			Handler:     handleUnoReverse,
		},
		{
			ID:          41,
			Name:        "Socialism",
			Description: "Take 90 from each of the Top 3 players and distribute it evenly among the Bottom 3 (includes you).",
			Rarity:      "Rare",
			Weight:      W_Rare,
			Handler:     handleSocialism,
		},
		{
			ID:          42,
			Name:        "Robin Hood",
			Description: "Steal 200 points from the richest player; keep 50 for yourself, give 150 to the poorest player.",
			Rarity:      "Rare",
			Weight:      W_Rare,
			Handler:     handleRobinHood,
		},
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
		// 	Description: "Randomize the points of the middle 3 players on the leaderboard. (swaps  players' points)",
		// 	Rarity:      "Rare",
		// 	Weight:      W_Rare,
		// 	Handler:     handleChaosDunk,
		// },
		{
			ID:                   64,
			Name:                 "Red Shells",
			Description:          "The 3 people directly in front of you on the leaderboard randomly lose between 25-50 points.",
			Rarity:               "Rare",
			Weight:               W_Rare,
			Handler:              handleRedShells,
			RoyaltyDiscordUserID: &[]string{"447827835797766144"}[0],
		},
		{
			ID:                   67,
			Name:                 "Factory Reset",
			Description:          "If you have less than 1000 points, you are reset to 1000 points.",
			Rarity:               "Rare",
			Weight:               W_Rare,
			Handler:              handleFactoryReset,
			RoyaltyDiscordUserID: &[]string{"447827835797766144"}[0],
		},
		{
			ID:                   68,
			Name:                 "Anti-Anti-Bet",
			Description:          "Choose a user. You place a new bet (even odds) that that user will lose their next bet they place. (Bet will be 100 points (if possible)).",
			Rarity:               "Rare",
			Weight:               W_Rare,
			Handler:              handleAntiAntiBet,
			RoyaltyDiscordUserID: &[]string{"238274131722764288"}[0],
		},
		{
			ID:          31,
			Name:        "Grand Larceny",
			Description: "Steal 150 points from a chosen user.",
			Rarity:      "Uncommon",
			Weight:      W_Uncommon,
			Handler:     handleGrandLarceny,
		},
	}

	// Add to deck
	*deck = append(*deck, rareCards...)
}
