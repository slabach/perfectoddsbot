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
	"time"

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

func PickRandomCard(hasSubscription bool, rarityMultiplier float64) *models.Card {
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

	higherRarities := map[string]bool{
		"Mythic": true,
		"Epic":   true,
		"Rare":   true,
	}

	totalWeight := 0
	for _, card := range eligibleCards {
		baseWeight := card.CardRarity.Weight

		if rarityMultiplier > 1.0 && higherRarities[card.CardRarity.Name] {
			adjustedWeight := int(float64(baseWeight) * rarityMultiplier)
			totalWeight += adjustedWeight
		} else {
			totalWeight += baseWeight
		}
	}

	if totalWeight == 0 {
		return nil
	}

	random := rand.Intn(totalWeight)

	cumulativeWeight := 0
	for _, card := range eligibleCards {
		baseWeight := card.CardRarity.Weight

		if rarityMultiplier > 1.0 && higherRarities[card.CardRarity.Name] {
			adjustedWeight := int(float64(baseWeight) * rarityMultiplier)
			cumulativeWeight += adjustedWeight
		} else {
			cumulativeWeight += baseWeight
		}

		if random < cumulativeWeight {
			return card
		}
	}

	return eligibleCards[0]
}

func PickCardByRarity(hasSubscription bool, rarityName string) *models.Card {
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

		if Deck[i].CardRarity.ID != 0 && Deck[i].CardRarity.Name == rarityName {
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

func GetUserRankFromTop5(db *gorm.DB, userID uint, guildID string) (rank int, distanceFromTop5 int, err error) {
	var user models.User
	if err := db.Where("id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return 6, 1, nil
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ?", guildID).Order("points desc").Find(&allUsers).Error; err != nil {
		return 0, 0, err
	}

	if len(allUsers) < 5 {
		return 1, 0, nil
	}

	for i, u := range allUsers {
		if u.ID == userID {
			rank = i + 1
			break
		}
	}

	if rank == 0 {
		return 6, 1, nil
	}

	if rank <= 5 {
		distanceFromTop5 = 0
	} else {
		distanceFromTop5 = rank - 5
	}

	return rank, distanceFromTop5, nil
}

func calculateRarityMultiplier(distanceFromTop5 int) float64 {
	if distanceFromTop5 <= 0 {
		return 1.0
	}

	multiplier := 1.0 + (float64(distanceFromTop5) * 0.1)

	if multiplier > 2.0 {
		multiplier = 2.0
	}

	return multiplier
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

	rand.Seed(time.Now().UnixNano())
	log.Printf("Loaded %d cards from database into deck (skipped %d cards without handlers)", len(Deck), skippedCount)
	return nil
}
