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
// hasSubscription indicates if the guild has a subscribed team (allowing access to premium cards)
func PickRandomCard(hasSubscription bool) *models.Card {
	if len(Deck) == 0 {
		return nil
	}

	// Filter deck based on subscription requirement
	var eligibleCards []*models.Card
	for i := range Deck {
		if Deck[i].RequiredSubscription && !hasSubscription {
			continue // Skip cards requiring subscription if guild doesn't have one
		}
		eligibleCards = append(eligibleCards, &Deck[i])
	}

	if len(eligibleCards) == 0 {
		return nil
	}

	// Calculate total weight of eligible cards
	totalWeight := 0
	for _, card := range eligibleCards {
		totalWeight += card.Weight
	}

	if totalWeight == 0 {
		return nil
	}

	// Generate random number between 0 and totalWeight
	random := rand.Intn(totalWeight)

	// Select card based on cumulative weight
	cumulativeWeight := 0
	for _, card := range eligibleCards {
		cumulativeWeight += card.Weight
		if random < cumulativeWeight {
			return card
		}
	}

	// Fallback (shouldn't reach here)
	return eligibleCards[0]
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
