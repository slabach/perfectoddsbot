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
	}

	// Add to deck
	*deck = append(*deck, rareCards...)
}
