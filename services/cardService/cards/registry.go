package cards

import "perfectOddsBot/models"

var codeCardDefsByID map[uint]*models.Card

func RegisterAllCards(deck *[]models.Card) {
	registerCommonCards(deck)
	registerUncommonCards(deck)
	registerRareCards(deck)
	registerEpicCards(deck)
	registerMythicCards(deck)

	codeCardDefsByID = make(map[uint]*models.Card, len(*deck))
	for i := range *deck {
		c := &(*deck)[i]
		codeCardDefsByID[c.ID] = c
	}
}

func IsPositiveFromCode(cardID uint) bool {
	def, ok := codeCardDefsByID[cardID]
	return ok && def != nil && def.IsPositive
}
