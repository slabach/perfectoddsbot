package cards

import "perfectOddsBot/models"

func registerMythicCards(deck *[]models.Card) {
	rareCards := []models.Card{
		{
			ID:          3,
			Name:        "The Grail",
			Description: "You discovered the Holy Grail! Win 50% of the pool!",
			Rarity:      "Rare",
			Weight:      W_Mythic,
			Handler:     handleGrail,
		},
		// {
		// 	ID:             43,
		// 	Name:           "Get Out of Jail Free",
		// 	Description:    "Nullifies the next lost bet completely.",
		// 	Rarity:         "Rare",
		// 	Weight:         W_Rare,
		// 	Handler:        handleGetOutOfJail,
		// 	AddToInventory: true,
		// },
		// {
		// 	ID:          52,
		// 	Name:        "JACKPOT",
		// 	Description: "You win 100% of the current Pool.",
		// 	Rarity:      "Mythic",
		// 	Weight:      W_Mythic,
		// 	Handler:     handleJackpot,
		// },
		// {
		// 	ID:                   66,
		// 	Name:                 "Uno Reverse",
		// 	Description:          "If you have any active bets, one of them randomly gets their option reversed.",
		// 	Rarity:               "Legendary",
		// 	Weight:               W_Epic,
		// 	Handler:              handlePurpleShell,
		// 	RoyaltyDiscordUserID: &[]string{"447827835797766144"}[0],
		// },
	}

	// Add to deck
	*deck = append(*deck, rareCards...)
}
