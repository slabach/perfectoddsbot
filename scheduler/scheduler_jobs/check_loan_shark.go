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

func CheckLoanShark(s *discordgo.Session, db *gorm.DB) error {
	var loans []models.UserInventory
	threeDaysAgo := time.Now().Add(-72 * time.Hour)

	err := db.Where("card_id = ? AND created_at < ?", cards.LoanSharkCardID, threeDaysAgo).Find(&loans).Error
	if err != nil {
		return err
	}

	if len(loans) == 0 {
		return nil
	}

	card := cardService.GetCardByID(cards.LoanSharkCardID)
	if card == nil {
		return fmt.Errorf("loan shark card definition not found")
	}

	for _, loan := range loans {
		var user models.User
		if err := db.First(&user, loan.UserID).Error; err != nil {
			continue
		}

		deduction := 600.0
		if user.Points < deduction {
			deduction = user.Points
		}

		user.Points -= deduction
		if user.Points < 0 {
			user.Points = 0
		}

		if err := db.Save(&user).Error; err != nil {
			continue
		}

		db.Delete(&loan)

		if err := cardService.NotifyCardPlayed(s, db, user, card); err != nil {
			fmt.Printf("Error notifying loan shark collection: %v\n", err)
		}
	}

	return nil
}
