package scheduler_jobs

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService"
	"perfectOddsBot/services/cardService/cards"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func CheckTheDevil(s *discordgo.Session, db *gorm.DB) error {
	var devilCards []models.UserInventory
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)

	err := db.Where("card_id = ? AND created_at < ? AND deleted_at IS NULL",
		cards.TheDevilCardID, sevenDaysAgo).Find(&devilCards).Error
	if err != nil {
		return err
	}

	if len(devilCards) == 0 {
		return nil
	}

	card := cardService.GetCardByID(cards.TheDevilCardID)
	if card == nil {
		return fmt.Errorf("the devil card definition not found")
	}

	for _, devilCard := range devilCards {
		var user models.User
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&user, devilCard.UserID).Error; err != nil {
				return err
			}

			if err := tx.Delete(&devilCard).Error; err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			fmt.Printf("Error processing The Devil for user %d: %v\n", devilCard.UserID, err)
			continue
		}

		if err := cardService.NotifyCardPlayed(s, db, user, card); err != nil {
			fmt.Printf("Error notifying The Devil expiration: %v\n", err)
		}
	}

	return nil
}
