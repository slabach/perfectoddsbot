package cardService

import (
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"

	"gorm.io/gorm"
)

func hasLuckyHorseshoeInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.LuckyHorseshoeCardID).
		Count(&count).Error
	return count > 0, err
}

func hasUnluckyCatInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.UnluckyCatCardID).
		Count(&count).Error
	return count > 0, err
}

func hasShieldInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.ShieldCardID).
		Count(&count).Error
	return count > 0, err
}

func hasMoonInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.TheMoonCardID).
		Count(&count).Error
	return count > 0, err
}

func getRandomUserForMoonFromCards(db *gorm.DB, guildID string, excludeUserIDs []uint) (string, error) {
	return cards.GetRandomUserForMoon(db, guildID, excludeUserIDs)
}

func hasGenerousDonationInInventory(db *gorm.DB, guildID string) (uint, error) {
	var inventory models.UserInventory
	err := db.Model(&models.UserInventory{}).
		Where("guild_id = ? AND card_id = ?", guildID, cards.GenerousDonationCardID).
		Limit(1).
		First(&inventory).Error

	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return inventory.UserID, nil
}
