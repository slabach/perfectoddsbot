package cardService

import (
	"math/rand"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"time"
)

var (
	Deck []models.Card
)

func init() {
	rand.Seed(time.Now().UnixNano())
	RegisterAllCards()
}

func RegisterAllCards() {
	cards.RegisterAllCards(&Deck)
}

func PickRandomCard(hasSubscription bool) *models.Card {
	if len(Deck) == 0 {
		return nil
	}

	var eligibleCards []*models.Card
	for i := range Deck {
		if Deck[i].RequiredSubscription && !hasSubscription {
			continue
		}
		eligibleCards = append(eligibleCards, &Deck[i])
	}

	if len(eligibleCards) == 0 {
		return nil
	}

	totalWeight := 0
	for _, card := range eligibleCards {
		totalWeight += card.Weight
	}

	if totalWeight == 0 {
		return nil
	}

	random := rand.Intn(totalWeight)

	cumulativeWeight := 0
	for _, card := range eligibleCards {
		cumulativeWeight += card.Weight
		if random < cumulativeWeight {
			return card
		}
	}

	return eligibleCards[0]
}

func GetCardByID(id int) *models.Card {
	for i := range Deck {
		if Deck[i].ID == uint(id) {
			return &Deck[i]
		}
	}
	return nil
}
