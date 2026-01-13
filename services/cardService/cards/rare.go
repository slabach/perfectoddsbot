package cards

import "perfectOddsBot/models"

func registerRareCards(deck *[]models.Card) {
	rareCards := []models.Card{}

	// Add to deck
	*deck = append(*deck, rareCards...)
}
