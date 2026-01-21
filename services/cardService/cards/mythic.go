package cards

import "perfectOddsBot/models"

func registerMythicCards(deck *[]models.Card) {
	mythicCards := []models.Card{
		{
			ID:          3,
			Code:        "GRA",
			Name:        "The Grail",
			Description: "You discovered the Holy Grail! Win 25% of the pool!",
			Rarity:      "Mythic",
			Weight:      W_Mythic,
			Handler:     handleGrail,
		},
		{
			ID:             43,
			Code:           "GOJ",
			Name:           "Get Out of Jail Free",
			Description:    "Nullifies the next lost bet completely.",
			Rarity:         "Mythic",
			Weight:         W_Mythic,
			Handler:        handleGetOutOfJail,
			AddToInventory: true,
		},
		// {
		// 	ID:          50,
		// 	Name:        "Market Crash",
		// 	Description: "All active bets currently placed are cancelled; all money bet on them goes to the Pool.",
		// 	Rarity:      "Mythic",
		// 	Weight:      W_Mythic,
		// 	Handler:     handleMarketCrash,
		// },
		{
			ID:          52,
			Code:        "POT",
			Name:        "JACKPOT",
			Description: "You win 50% of the current Pool.",
			Rarity:      "Mythic",
			Weight:      W_Mythic,
			Handler:     handleJackpot,
		},
	}

	*deck = append(*deck, mythicCards...)
}
