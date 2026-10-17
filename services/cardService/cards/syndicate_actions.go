package cards

import (
	"fmt"
	"math/rand"
	"time"

	"perfectOddsBot/models"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func heldCardResult(name string) (*models.CardResult, error) {
	return &models.CardResult{
		Message: fmt.Sprintf("%s was added to your inventory. Play it when you want.", name),
	}, nil
}

func handleBrassKnuckles(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Brass Knuckles")
}
func handleWateredDownHooch(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Watered-Down Hooch")
}
func handleLookoutBoy(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Lookout Boy")
}
func handleLoadedDice(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Loaded Dice")
}
func handlePointShaver(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Point Shaver")
}
func handleProtectionRacket(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Protection Racket")
}
func handleCookedBooks(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Cooked Books")
}
func handleBackroomWiretap(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Backroom Wiretap")
}
func handleConcreteShoes(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Concrete Shoes")
}
func handleTheFixIsIn(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("The Fix Is In")
}
func handleBagmansCut(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Bagman's Cut")
}
func handleSpeakeasyParlay(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Speakeasy Parlay")
}
func handleLeveragedBuyout(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("Leveraged Buyout")
}
func handleTheDonsFavor(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return heldCardResult("The Don's Favor")
}

// IsHeldCopy is a card waiting to be played. Effect rows carry a target, an expiry, or a stored roll.
func IsHeldCopy(item models.UserInventory) bool {
	return item.TargetBetID == nil &&
		item.TargetEntryID == nil &&
		item.TargetParlayID == nil &&
		item.TargetUserID == nil &&
		item.ExpiresAt == nil &&
		item.BetAmount == 0 &&
		item.TimesApplied == 0
}

func ConsumeHeldCopy(tx *gorm.DB, userDBID uint, guildID string, cardID uint) error {
	var item models.UserInventory
	err := tx.Where(`user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL
		AND target_bet_id IS NULL AND target_entry_id IS NULL AND target_parlay_id IS NULL
		AND target_user_id IS NULL AND expires_at IS NULL AND bet_amount = 0 AND times_applied = 0`,
		userDBID, guildID, cardID).
		First(&item).Error
	if err != nil {
		return err
	}
	return tx.Delete(&item).Error
}

func ProtectedByCookedBooks(db *gorm.DB, userDBID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL AND expires_at IS NOT NULL AND expires_at > ?",
			userDBID, guildID, CookedBooksCardID, time.Now()).
		Count(&count).Error
	return count > 0, err
}

func ConcreteShoesActive(db *gorm.DB, userDBID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL AND expires_at IS NOT NULL AND expires_at > ?",
			userDBID, guildID, ConcreteShoesCardID, time.Now()).
		Count(&count).Error
	return count > 0, err
}

func HasHoochCurse(db *gorm.DB, userDBID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL AND target_user_id IS NOT NULL",
			userDBID, guildID, WateredDownHoochCardID).
		Count(&count).Error
	return count > 0, err
}

func ConsumeHoochCurse(tx *gorm.DB, userDBID uint, guildID string) error {
	var item models.UserInventory
	err := tx.Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL AND target_user_id IS NOT NULL",
		userDBID, guildID, WateredDownHoochCardID).
		First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	return tx.Delete(&item).Error
}

func handleStreetTax(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	err := db.Transaction(func(tx *gorm.DB) error {
		var drawer models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&drawer).Error; err != nil {
			return err
		}
		var top []models.User
		if err := tx.Where("guild_id = ?", guildID).Order("points DESC").Limit(3).Find(&top).Error; err != nil {
			return err
		}
		var candidates []models.User
		for _, u := range top {
			if u.ID == drawer.ID || u.Points <= 0 {
				continue
			}
			candidates = append(candidates, u)
		}
		if len(candidates) == 0 {
			result = &models.CardResult{Message: "Street Tax fizzles. Nobody in the top 3 can pay."}
			return nil
		}
		target := candidates[rand.Intn(len(candidates))]
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, target.ID).Error; err != nil {
			return err
		}
		protected, err := blockPointSteal(tx, target.ID, guildID)
		if err != nil {
			return err
		}
		if protected != "" {
			result = &models.CardResult{
				Message:      fmt.Sprintf("Street Tax fizzles. %s's %s keeps the ledger shut.", mention(target.DiscordID), protected),
				TargetUserID: &target.DiscordID,
			}
			return nil
		}
		take := StreetTaxAmount
		if target.Points < take {
			take = target.Points
		}
		if take <= 0 {
			result = &models.CardResult{Message: "Street Tax fizzles. The mark is broke."}
			return nil
		}
		target.Points -= take
		drawer.Points += take
		if err := tx.Save(&target).Error; err != nil {
			return err
		}
		if err := tx.Save(&drawer).Error; err != nil {
			return err
		}
		result = &models.CardResult{
			Message:           fmt.Sprintf("Street Tax collects %.0f from %s.", take, mention(target.DiscordID)),
			PointsDelta:       take,
			TargetUserID:      &target.DiscordID,
			TargetPointsDelta: -take,
		}
		return nil
	})
	return result, err
}

func mention(discordID string) string {
	return "<@" + discordID + ">"
}

// blockPointSteal checks Cooked Books, then Moon, then Redshirt and Shield.
// Books do not consume the other protections. The returned label is empty when the steal lands.
func blockPointSteal(tx *gorm.DB, userDBID uint, guildID string) (string, error) {
	books, err := ProtectedByCookedBooks(tx, userDBID, guildID)
	if err != nil {
		return "", err
	}
	if books {
		return "Cooked Books", nil
	}
	moon, err := CheckAndConsumeMoon(tx, userDBID, guildID)
	if err != nil {
		return "", err
	}
	if moon {
		return "Moon", nil
	}
	blocked, protection, err := CheckAndConsumeShieldOrRedshirt(tx, userDBID, guildID)
	if err != nil || !blocked {
		return "", err
	}
	return protection, nil
}

// RollLoadedDice returns a hundredth between 0.85 and 1.25 inclusive.
func RollLoadedDice() float64 {
	return LoadedDiceMin + float64(rand.Intn(41))*0.01
}
