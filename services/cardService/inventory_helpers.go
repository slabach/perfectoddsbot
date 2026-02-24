package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"time"

	"gorm.io/gorm"
)

func processRoyaltyPayment(tx *gorm.DB, card *models.Card, royaltyGuildID string) error {
	if card.RoyaltyDiscordUserID == nil {
		return nil
	}

	var royaltyAmount float64
	if card.CardRarity.ID != 0 {
		royaltyAmount = card.CardRarity.Royalty
	} else {
		royaltyAmount = 0.5 // Default to Common royalty
	}

	var royaltyGuild models.Guild
	guildResult := tx.Where("guild_id = ?", royaltyGuildID).First(&royaltyGuild)
	if guildResult.Error != nil {
		return fmt.Errorf("error fetching royalty guild: %v", guildResult.Error)
	}

	var royaltyUser models.User
	result := tx.First(&royaltyUser, models.User{
		DiscordID: *card.RoyaltyDiscordUserID,
		GuildID:   royaltyGuildID,
	})
	if result.Error != nil {
		return fmt.Errorf("error fetching royalty user: %v", result.Error)
	}

	if err := tx.Model(&royaltyUser).UpdateColumn("points", gorm.Expr("points + ?", royaltyAmount)).Error; err != nil {
		return fmt.Errorf("error saving royalty user: %v", err)
	}

	return nil
}

func GetExpiresAtForNewCard(cardID uint) *time.Time {
	now := time.Now()
	switch cardID {
	case cards.RedshirtCardID:
		t := now.Add(2 * time.Hour)
		return &t
	case cards.VampireCardID:
		t := now.Add(24 * time.Hour)
		return &t
	case cards.TheDevilCardID:
		t := now.Add(7 * 24 * time.Hour)
		return &t
	case cards.HomeFieldAdvantageCardID:
		return nil
	default:
		return nil
	}
}

func addCardToInventory(db *gorm.DB, userID uint, guildID string, cardID uint, cardCode string, expiresAt *time.Time) error {
	inventory := models.UserInventory{
		UserID:    userID,
		GuildID:   guildID,
		CardID:    cardID,
		CardCode:  cardCode,
		ExpiresAt: expiresAt,
	}
	return db.Create(&inventory).Error
}

func processTagCards(tx *gorm.DB, guildID string) error {
	now := time.Now()
	expirationTime := now.Add(-12 * time.Hour)

	var tagCards []models.UserInventory
	if err := tx.Where("guild_id = ? AND card_id = ? AND deleted_at IS NULL", guildID, cards.TagCardID).
		Find(&tagCards).Error; err != nil {
		return err
	}

	if len(tagCards) == 0 {
		return nil
	}

	userPointsMap := make(map[uint]bool)
	var expiredCards []models.UserInventory
	var userIDsToUpdate []uint

	for _, tagCard := range tagCards {
		if tagCard.CreatedAt.Before(expirationTime) {
			expiredCards = append(expiredCards, tagCard)
		} else {
			if !userPointsMap[tagCard.UserID] {
				userIDsToUpdate = append(userIDsToUpdate, tagCard.UserID)
				userPointsMap[tagCard.UserID] = true
			}
		}
	}

	if len(userIDsToUpdate) > 0 {
		if err := tx.Model(&models.User{}).
			Where("id IN ? AND guild_id = ?", userIDsToUpdate, guildID).
			UpdateColumn("points", gorm.Expr("points + 1.0")).Error; err != nil {
			return err
		}
	}

	for _, expiredCard := range expiredCards {
		if err := tx.Delete(&expiredCard).Error; err != nil {
			return err
		}
	}

	return nil
}

func getUserInventory(db *gorm.DB, userID uint, guildID string) ([]models.UserInventory, error) {
	var inventory []models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ?", userID, guildID).Find(&inventory).Error
	return inventory, err
}
