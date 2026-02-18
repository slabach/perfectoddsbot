package cardService

import (
	"math/rand"
	"perfectOddsBot/models"

	"gorm.io/gorm"
)

type TradeableInventoryItem struct {
	Inventory models.UserInventory
	Card      *models.Card
}

func GetEligibleUsersWithTradeableCards(db *gorm.DB, guildID string, excludeDrawerDiscordID string) ([]models.User, error) {
	var userIDs []uint
	err := db.Table("user_inventories").
		Select("DISTINCT user_inventories.user_id").
		Joins("JOIN cards ON cards.id = user_inventories.card_id").
		Joins("JOIN card_rarities ON card_rarities.id = cards.rarity_id AND card_rarities.name != ?", "Mythic").
		Where("user_inventories.guild_id = ? AND user_inventories.deleted_at IS NULL", guildID).
		Pluck("user_id", &userIDs).Error
	if err != nil {
		return nil, err
	}
	if len(userIDs) == 0 {
		return nil, nil
	}
	var users []models.User
	err = db.Where("guild_id = ? AND deleted_at IS NULL AND discord_id != ? AND id IN ?", guildID, excludeDrawerDiscordID, userIDs).Find(&users).Error
	return users, err
}

func HasTradeableCard(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Table("user_inventories").
		Joins("JOIN cards ON cards.id = user_inventories.card_id").
		Joins("JOIN card_rarities ON card_rarities.id = cards.rarity_id AND card_rarities.name != ?", "Mythic").
		Where("user_inventories.user_id = ? AND user_inventories.guild_id = ? AND user_inventories.deleted_at IS NULL", userID, guildID).
		Count(&count).Error
	return count > 0, err
}

func GetTradeableInventoryForUser(db *gorm.DB, userID uint, guildID string) ([]TradeableInventoryItem, error) {
	var inventory []models.UserInventory
	if err := db.Where("user_id = ? AND guild_id = ? AND deleted_at IS NULL", userID, guildID).Find(&inventory).Error; err != nil {
		return nil, err
	}
	var result []TradeableInventoryItem
	for _, inv := range inventory {
		card := GetCardByID(inv.CardID)
		if card == nil {
			continue
		}
		if card.CardRarity.ID != 0 && card.CardRarity.Name == "Mythic" {
			continue
		}
		result = append(result, TradeableInventoryItem{Inventory: inv, Card: card})
	}
	return result, nil
}

func PickWeightedTradeable(items []TradeableInventoryItem) *TradeableInventoryItem {
	if len(items) == 0 {
		return nil
	}
	if len(items) == 1 {
		return &items[0]
	}
	totalWeight := 0
	for i := range items {
		w := 1
		if items[i].Card != nil && items[i].Card.CardRarity.ID != 0 && items[i].Card.CardRarity.Weight > 0 {
			w = items[i].Card.CardRarity.Weight
		}
		totalWeight += w
	}
	if totalWeight <= 0 {
		return &items[0]
	}
	random := rand.Intn(totalWeight)
	cumulative := 0
	for i := range items {
		w := 1
		if items[i].Card != nil && items[i].Card.CardRarity.ID != 0 && items[i].Card.CardRarity.Weight > 0 {
			w = items[i].Card.CardRarity.Weight
		}
		cumulative += w
		if random < cumulative {
			return &items[i]
		}
	}
	return &items[len(items)-1]
}
