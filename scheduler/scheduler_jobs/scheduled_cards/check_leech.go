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

func CheckLeech(s *discordgo.Session, db *gorm.DB) error {
	now := time.Now()
	twelveHoursAgo := now.Add(-12 * time.Hour)

	var activeLeeches []models.UserInventory
	err := db.Where("card_id = ? AND created_at >= ? AND created_at < ? AND deleted_at IS NULL", cards.LeechCardID, twelveHoursAgo, now).Find(&activeLeeches).Error
	if err != nil {
		return err
	}

	for _, leech := range activeLeeches {
		err := db.Transaction(func(tx *gorm.DB) error {
			var leechHolder models.User
			if err := tx.First(&leechHolder, leech.UserID).Error; err != nil {
				return err
			}

			var richestPlayer models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("guild_id = ? AND id != ?", leech.GuildID, leechHolder.ID).
				Order("points DESC").
				First(&richestPlayer).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return nil
				}
				return err
			}

			siphonAmount := richestPlayer.Points * 0.01
			if siphonAmount <= 0 {
				return nil
			}

			// Shield/moon for richest player (victim of Leech)
			moonRedirected, err := cards.CheckAndConsumeMoon(tx, richestPlayer.ID, leech.GuildID)
			if err != nil {
				return err
			}
			if moonRedirected {
				randomDiscordID, err := cards.GetRandomUserForMoon(tx, leech.GuildID, []uint{richestPlayer.ID, leechHolder.ID})
				if err != nil {
					cards.CheckAndConsumeShield(tx, richestPlayer.ID, leech.GuildID)
					return nil
				}
				var randomUser models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("discord_id = ? AND guild_id = ?", randomDiscordID, leech.GuildID).
					First(&randomUser).Error; err != nil {
					return err
				}
				actualSiphon := randomUser.Points * 0.01
				if actualSiphon <= 0 {
					return nil
				}
				randomUser.Points -= actualSiphon
				if randomUser.Points < 0 {
					randomUser.Points = 0
				}
				if err := tx.Save(&randomUser).Error; err != nil {
					return err
				}
				leechHolder.Points += actualSiphon
				if err := tx.Save(&leechHolder).Error; err != nil {
					return err
				}
				return nil
			}

			blocked, err := cards.CheckAndConsumeShield(tx, richestPlayer.ID, leech.GuildID)
			if err != nil {
				return err
			}
			if blocked {
				return nil
			}

			richestPlayer.Points -= siphonAmount
			if richestPlayer.Points < 0 {
				richestPlayer.Points = 0
			}
			if err := tx.Save(&richestPlayer).Error; err != nil {
				return err
			}

			leechHolder.Points += siphonAmount
			if err := tx.Save(&leechHolder).Error; err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			fmt.Printf("Error processing leech for user %d: %v\n", leech.UserID, err)
			continue
		}
	}

	var expiredLeeches []models.UserInventory
	err = db.Where("card_id = ? AND created_at < ? AND deleted_at IS NULL", cards.LeechCardID, twelveHoursAgo).Find(&expiredLeeches).Error
	if err != nil {
		return err
	}

	if len(expiredLeeches) == 0 {
		return nil
	}

	card := cardService.GetCardByID(cards.LeechCardID)
	if card == nil {
		return fmt.Errorf("leech card definition not found")
	}

	for _, leech := range expiredLeeches {
		var user models.User
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.First(&user, leech.UserID).Error; err != nil {
				return err
			}

			if err := tx.Delete(&leech).Error; err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			fmt.Printf("Error processing leech expiration for user %d: %v\n", leech.UserID, err)
			continue
		}

		expirationMessage := fmt.Sprintf("<@%s>'s **%s** has expired", user.DiscordID, card.Name)
		if err := cardService.NotifyCardPlayedWithMessage(s, db, user, card, expirationMessage); err != nil {
			fmt.Printf("Error notifying leech expiration: %v\n", err)
		}
	}

	return nil
}
