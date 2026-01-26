package cards

import "perfectOddsBot/models"

func RegisterAllCards(deck *[]models.Card) {
	registerCommonCards(deck)
	registerUncommonCards(deck)
	registerRareCards(deck)
	registerEpicCards(deck)
	registerMythicCards(deck)
}
