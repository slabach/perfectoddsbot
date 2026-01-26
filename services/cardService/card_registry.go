package cardService

import (
	"log"
	"math/rand"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"gorm.io/gorm"
)

var (
	Deck            []models.Card
	cardMap         map[uint]*models.Card
	handlerRegistry map[string]models.CardHandler
	deckMu          sync.RWMutex
)

func init() {
	populateHandlerRegistry()
}

func populateHandlerRegistry() {
	handlerRegistry = make(map[string]models.CardHandler)

	var codeDeck []models.Card
	cards.RegisterAllCards(&codeDeck)

	extractHandlerName := func(handler models.CardHandler) string {
		if handler == nil {
			return ""
		}
		funcValue := reflect.ValueOf(handler)
		if !funcValue.IsValid() || funcValue.IsNil() {
			return ""
		}
		funcPtr := funcValue.Pointer()
		if funcPtr == 0 {
			return ""
		}
		fn := runtime.FuncForPC(funcPtr)
		if fn == nil {
			return ""
		}
		fullName := fn.Name()
		parts := strings.Split(fullName, ".")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return fullName
	}

	for _, card := range codeDeck {
		if card.Handler != nil {
			handlerName := extractHandlerName(card.Handler)
			if handlerName != "" {
				handlerRegistry[handlerName] = card.Handler
			}
		}
	}
}

func RegisterAllCards() {
	populateHandlerRegistry()
}

func PickRandomCard(hasSubscription bool) *models.Card {
	deckMu.RLock()
	if len(Deck) == 0 {
		deckMu.RUnlock()
		return nil
	}

	var eligibleCards []*models.Card
	for i := range Deck {
		if Deck[i].RequiredSubscription && !hasSubscription {
			continue
		}

		if Deck[i].CardRarity.ID != 0 {
			eligibleCards = append(eligibleCards, &Deck[i])
		}
	}
	deckMu.RUnlock()

	if len(eligibleCards) == 0 {
		return nil
	}

	totalWeight := 0
	for _, card := range eligibleCards {
		totalWeight += card.CardRarity.Weight
	}

	if totalWeight == 0 {
		return nil
	}

	random := rand.Intn(totalWeight)

	cumulativeWeight := 0
	for _, card := range eligibleCards {
		cumulativeWeight += card.CardRarity.Weight
		if random < cumulativeWeight {
			return card
		}
	}

	return eligibleCards[0]
}

func GetCardByID(id uint) *models.Card {
	deckMu.RLock()
	defer deckMu.RUnlock()
	return cardMap[id]
}

func LoadDeckFromDB(db *gorm.DB) error {
	var dbCards []models.Card
	err := db.Where("active = ? AND rarity_id IS NOT NULL", true).
		Preload("CardRarity").
		Preload("Options").
		Find(&dbCards).Error

	if err != nil {
		log.Printf("Error loading cards from database: %v", err)
		return err
	}

	deckMu.Lock()
	defer deckMu.Unlock()

	Deck = make([]models.Card, 0, len(dbCards))
	cardMap = make(map[uint]*models.Card, len(dbCards))

	skippedCount := 0
	for i := range dbCards {
		card := &dbCards[i]

		if card.HandlerName != "" {
			if handler, exists := handlerRegistry[card.HandlerName]; exists {
				card.Handler = handler
			} else {
				log.Printf("Warning: Handler '%s' not found in registry for card %d (%s), skipping", card.HandlerName, card.ID, card.Name)
				skippedCount++
				continue
			}
		} else {
			log.Printf("Warning: Card %d (%s) has no handler name, skipping", card.ID, card.Name)
			skippedCount++
			continue
		}

		Deck = append(Deck, *card)

		cardMap[card.ID] = &Deck[len(Deck)-1]
	}

	log.Printf("Loaded %d cards from database into deck (skipped %d cards without handlers)", len(Deck), skippedCount)
	return nil
}
