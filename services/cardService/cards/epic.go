package cards

import "perfectOddsBot/models"

func registerEpicCards(deck *[]models.Card) {
	epicCards := []models.Card{}

	// Add to deck
	*deck = append(*deck, epicCards...)
}
