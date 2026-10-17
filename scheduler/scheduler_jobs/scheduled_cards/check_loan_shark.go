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
		collected := false
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, loan.UserID).Error; err != nil {
				return err
			}

			protected, err := cards.ProtectedByCookedBooks(tx, user.ID, user.GuildID)
			if err != nil {
				return err
			}
			if protected {
				return nil
			}

			deduction := 600.0
			if user.Points < deduction {
				deduction = user.Points
			}

			user.Points -= deduction
			if user.Points < 0 {
				user.Points = 0
			}

			if err := tx.Save(&user).Error; err != nil {
				return err
			}

			if err := tx.Delete(&loan).Error; err != nil {
				return err
			}

			collected = true
			return nil
		})

		if err != nil {
			fmt.Printf("Error processing loan shark for user %d: %v\n", loan.UserID, err)
			continue
		}
		if !collected {
			continue
		}

		expirationMessage := fmt.Sprintf("<@%s>'s **%s** has expired", user.DiscordID, card.Name)
		if err := cardService.NotifyCardPlayedWithMessage(s, db, user, card, expirationMessage); err != nil {
			fmt.Printf("Error notifying loan shark collection: %v\n", err)
		}
	}

	return nil
}
