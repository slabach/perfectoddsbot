package cards

import "perfectOddsBot/models"

func registerMythicCards(deck *[]models.Card) {
	rareCards := []models.Card{{
		ID:          3,
		Name:        "The Grail",
		Description: "You discovered the Holy Grail! Win 50% of the pool!",
		Rarity:      "Rare",
		Weight:      W_Mythic,
		Handler:     handleGrail,
	},
	}

	// Add to deck
	*deck = append(*deck, rareCards...)
}
