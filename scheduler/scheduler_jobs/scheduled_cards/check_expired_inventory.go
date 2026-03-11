package scheduler_jobs

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func CheckExpiredInventory(s *discordgo.Session, db *gorm.DB) error {
	now := time.Now()
	var items []models.UserInventory
	err := db.Where("expires_at IS NOT NULL AND expires_at < ? AND deleted_at IS NULL", now).Find(&items).Error
	if err != nil {
		return err
	}
	for _, item := range items {
		var user models.User
		if err := db.First(&user, item.UserID).Error; err != nil {
			fmt.Printf("Error loading user %d for expired inventory %d: %v\n", item.UserID, item.ID, err)
			continue
		}
		card := cardService.GetCardByID(item.CardID)
		if card != nil {
			expirationMessage := fmt.Sprintf("<@%s>'s **%s** has expired", user.DiscordID, card.Name)
			if err := cardService.NotifyCardPlayedWithMessage(s, db, user, card, expirationMessage); err != nil {
				fmt.Printf("Error notifying expired card %d for user %d: %v\n", item.CardID, item.UserID, err)
			}
		}
		if err := db.Delete(&item).Error; err != nil {
			fmt.Printf("Error soft-deleting expired inventory %d: %v\n", item.ID, err)
		}
	}
	return nil
}
