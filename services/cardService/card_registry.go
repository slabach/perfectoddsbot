package cardService

import (
	"math/rand"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"time"
)

var (
	// Deck contains all registered cards
	Deck []models.Card
)

func init() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())
	// Register all cards
	RegisterAllCards()
}

// RegisterAllCards registers all card types
func RegisterAllCards() {
	cards.RegisterAllCards(&Deck)
}

// PickRandomCard selects a random card based on weighted distribution
func PickRandomCard() *models.Card {
	if len(Deck) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0
	for _, card := range Deck {
		totalWeight += card.Weight
	}

	if totalWeight == 0 {
		return nil
	}

	// Generate random number between 0 and totalWeight
	random := rand.Intn(totalWeight)

	// Select card based on cumulative weight
	cumulativeWeight := 0
	for i := range Deck {
		cumulativeWeight += Deck[i].Weight
		if random < cumulativeWeight {
			return &Deck[i]
		}
	}

	// Fallback (shouldn't reach here)
	return &Deck[0]
}

// GetCardByID returns a card by its ID, or nil if not found
func GetCardByID(id int) *models.Card {
	for i := range Deck {
		if Deck[i].ID == id {
			return &Deck[i]
		}
	}
	return nil
}
