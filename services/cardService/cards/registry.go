package cards

import "perfectOddsBot/models"

// RegisterAllCards registers all cards into the deck
func RegisterAllCards(deck *[]models.Card) {
	registerCommonCards(deck)
	registerUncommonCards(deck)
	registerRareCards(deck)
	registerEpicCards(deck)
	registerMythicCards(deck)
}
