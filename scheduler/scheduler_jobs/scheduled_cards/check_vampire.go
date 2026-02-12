package scheduler_jobs

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService"
	"perfectOddsBot/services/cardService/cards"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func CheckVampire(s *discordgo.Session, db *gorm.DB) error {
	var vampires []models.UserInventory
	twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)

	err := db.Where("card_id = ? AND created_at < ?", cards.VampireCardID, twentyFourHoursAgo).Find(&vampires).Error
	if err != nil {
		return err
	}

	if len(vampires) == 0 {
		return nil
	}

	card := cardService.GetCardByID(cards.VampireCardID)
	if card == nil {
		return fmt.Errorf("vampire card definition not found")
	}

	for _, vampire := range vampires {
		var user models.User
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.First(&user, vampire.UserID).Error; err != nil {
				return err
			}

			if err := tx.Delete(&vampire).Error; err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			fmt.Printf("Error processing vampire expiration for user %d: %v\n", vampire.UserID, err)
			continue
		}

		if err := cardService.NotifyCardPlayed(s, db, user, card); err != nil {
			fmt.Printf("Error notifying vampire expiration: %v\n", err)
		}
	}

	return nil
}
