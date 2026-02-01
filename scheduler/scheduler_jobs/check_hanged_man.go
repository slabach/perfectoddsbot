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

func CheckHangedMan(s *discordgo.Session, db *gorm.DB) error {
	var hangedManCards []models.UserInventory
	twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)

	err := db.Where("card_id = ? AND created_at < ? AND deleted_at IS NULL",
		cards.TheHangedManCardID, twentyFourHoursAgo).Find(&hangedManCards).Error
	if err != nil {
		return err
	}

	if len(hangedManCards) == 0 {
		return nil
	}

	card := cardService.GetCardByID(cards.TheHangedManCardID)
	if card == nil {
		return fmt.Errorf("hanged man card definition not found")
	}

	for _, hangedManCard := range hangedManCards {
		var user models.User
		var guild models.Guild
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&user, hangedManCard.UserID).Error; err != nil {
				return err
			}

			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("guild_id = ?", hangedManCard.GuildID).
				First(&guild).Error; err != nil {
				return err
			}

			gainAmount := 400.0
			if guild.Pool < gainAmount {
				gainAmount = guild.Pool
			}

			user.Points += gainAmount
			guild.Pool -= gainAmount

			if err := tx.Save(&user).Error; err != nil {
				return err
			}

			if err := tx.Save(&guild).Error; err != nil {
				return err
			}

			if err := tx.Delete(&hangedManCard).Error; err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			fmt.Printf("Error processing hanged man for user %d: %v\n", hangedManCard.UserID, err)
			continue
		}

		if err := cardService.NotifyCardPlayed(s, db, user, card); err != nil {
			fmt.Printf("Error notifying hanged man payout: %v\n", err)
		}
	}

	return nil
}
