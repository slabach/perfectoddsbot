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
		// {
		// 	ID:                   55,
		// 	Code:                 "SSW",
		// 	Name:                 "Supermarket Sweep",
		// 	Description:          "For the next 30 seconds, you can draw as many cards as you want without paying for them.",
		// 	Handler:              handleSupermarketSweep,
		// 	AddToInventory:       true,
		// 	RoyaltyDiscordUserID: &[]string{"313553928115716097"}[0],
		// },
		{
			ID:          98,
			Code:        "DTH",
			Name:        "Death (🔮)",
			Description: "Transformation. Every 'positive' inventory card (eg. Shield/Double Down/etc) currently held by any player (except Mythics) is destroyed. 100 points are drained from the pool for each card destroyed.",
			Handler:     handleDeath,
			Expansion:   "Tarot",
		},
		{
			ID:          102,
			Code:        "TOW",
			Name:        "The Tower (🔮)",
			Description: "Sudden Collapse. The pool is instantly reduced by 75%. Every player loses 50 points as the debris settles.",
			Handler:     handleTheTower,
			Expansion:   "Tarot",
		},
		{
			ID:          103,
			Code:        "WOR",
			Name:        "The World (🔮)",
			Description: "Completion. Win 10% of the pool and immediately resolve one of your open bets as a win.",
			Handler:     handleTheWorld,
			Expansion:   "Tarot",
		},
	}

	*deck = append(*deck, mythicCards...)
}
