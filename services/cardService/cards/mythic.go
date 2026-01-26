package cards

import "perfectOddsBot/models"

func registerMythicCards(deck *[]models.Card) {
	mythicCards := []models.Card{
		{
			ID:          3,
			Code:        "GRA",
			Name:        "The Grail",
			Description: "You discovered the Holy Grail! Win 25% of the pool!",
			Handler:     handleGrail,
		},
		{
			ID:             43,
			Code:           "GOJ",
			Name:           "Get Out of Jail Free",
			Description:    "Nullifies the next lost bet completely.",
			Handler:        handleGetOutOfJail,
			AddToInventory: true,
		},
		// {
		// 	ID:          50,
		// 	Code:        "MCR",
		// 	Name:        "Market Crash",
		// 	Description: "All active bets currently placed are cancelled; all money bet on them goes to the Pool.",
		// 	Handler:     handleMarketCrash,
		// },
		// {
		// 	ID:          51,
		// 	Code:        "TTM",
		// 	Name:        "To the Moon 🚀",
		// 	Description: "All active bets currently placed are resolved as wins.",
		// 	Handler:     handleToTheMoon,
		// },
		{
			ID:          52,
			Code:        "POT",
			Name:        "JACKPOT",
			Description: "You win 50% of the current Pool.",
			Handler:     handleJackpot,
		},
	}

	*deck = append(*deck, mythicCards...)
}
