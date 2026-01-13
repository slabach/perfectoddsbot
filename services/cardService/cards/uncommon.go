package cards

import "perfectOddsBot/models"

func registerUncommonCards(deck *[]models.Card) {
	uncommonCards := []models.Card{
		{
			ID:          4,
			Name:        "The Pickpocket",
			Description: "Choose a user. Steal 5% of their points.",
			Rarity:      "Epic",
			Weight:      2,
			Handler:     handlePickpocket,
		},
	}

	// Add to deck
	*deck = append(*deck, uncommonCards...)
}
