package cards

import "perfectOddsBot/models"

func registerRareCards(deck *[]models.Card) {
	rareCards := []models.Card{
		// {
		// 	ID:                   38,
		// 	Name:                 "The Blue Shell",
		// 	Description:          "The player in 1st place loses 20% of their total points to the Pool.",
		// 	Rarity:               "Rare",
		// 	Weight:               W_Rare,
		// 	Handler:              handleBlueShell,
		// 	RoyaltyDiscordUserID: &[]string{"698712210515558432"}[0],
		// },
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
	}

	// Add to deck
	*deck = append(*deck, rareCards...)
}
