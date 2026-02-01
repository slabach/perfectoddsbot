package cards

import "perfectOddsBot/models"

// codeCardDefsByID holds code-defined card definitions (including IsPositive).
// Populated by RegisterAllCards. Used when DB may not have synced fields (e.g. IsPositive).
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

// IsPositiveFromCode returns the code-defined IsPositive for a card ID.
// Returns false if the card is not in the code deck (e.g. unknown or DB-only card).
func IsPositiveFromCode(cardID uint) bool {
	def, ok := codeCardDefsByID[cardID]
	return ok && def != nil && def.IsPositive
}
