package cards

import "perfectOddsBot/models"

func registerCommonCards(deck *[]models.Card) {
	commonCards := []models.Card{
		{
			ID:          1,
			Name:        "The Dud",
			Description: "A blank card. Nothing happens.",
			Rarity:      "Common",
			Weight:      20,
			Handler:     handleDud,
		},
		{
			ID:          2,
			Name:        "The Penny",
			Description: "You found a penny on the ground. It's worth something, right?",
			Rarity:      "Common",
			Weight:      25,
			Handler:     handlePenny,
		},
		{
			ID:          5,
			Name:        "Papercut",
			Description: "You cut your finger drawing the card. Lose 10 points.",
			Rarity:      "Common",
			Weight:      25,
			Handler:     handlePapercut,
		},
		{
			ID:          6,
			Name:        "Vending Machine",
			Description: "You found loose change in the return slot. Gain 25 points.",
			Rarity:      "Common",
			Weight:      25,
			Handler:     handleVendingMachine,
		},
	}

	// Add to deck
	*deck = append(*deck, commonCards...)
}
