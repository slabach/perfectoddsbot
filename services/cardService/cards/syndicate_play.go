package cards

import (
	"fmt"
	"math/rand"
	"time"

	"perfectOddsBot/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func effectBase(user models.User, cardID uint, code string) models.UserInventory {
	return models.UserInventory{
		UserID:   user.ID,
		GuildID:  user.GuildID,
		CardID:   cardID,
		CardCode: code,
	}
}

func StartCookedBooks(tx *gorm.DB, user models.User) error {
	if err := ConsumeHeldCopy(tx, user.ID, user.GuildID, CookedBooksCardID); err != nil {
		return err
	}
	until := time.Now().Add(24 * time.Hour)
	row := effectBase(user, CookedBooksCardID, "CKB")
	row.ExpiresAt = &until
	return tx.Create(&row).Error
}

func StartBagman(tx *gorm.DB, user models.User) error {
	if err := ConsumeHeldCopy(tx, user.ID, user.GuildID, BagmansCutCardID); err != nil {
		return err
	}
	until := time.Now().Add(12 * time.Hour)
	return tx.Model(&models.Guild{}).Where("guild_id = ?", user.GuildID).Updates(map[string]interface{}{
		"bagman_active_until":      until,
		"bagman_holder_discord_id": user.DiscordID,
	}).Error
}

func PlaceCurseOnTarget(tx *gorm.DB, caster models.User, target models.User, cardID uint, code string, expires *time.Time) error {
	if err := ConsumeHeldCopy(tx, caster.ID, caster.GuildID, cardID); err != nil {
		return err
	}
	row := effectBase(target, cardID, code)
	row.TargetUserID = &caster.DiscordID
	row.ExpiresAt = expires
	return tx.Create(&row).Error
}

func PlaceRacket(tx *gorm.DB, caster models.User, target models.User) error {
	if err := ConsumeHeldCopy(tx, caster.ID, caster.GuildID, ProtectionRacketCardID); err != nil {
		return err
	}
	row := effectBase(caster, ProtectionRacketCardID, "PRK")
	row.TargetUserID = &target.DiscordID
	return tx.Create(&row).Error
}

func PlaceDon(tx *gorm.DB, caster models.User, target models.User) error {
	if err := ConsumeHeldCopy(tx, caster.ID, caster.GuildID, TheDonsFavorCardID); err != nil {
		return err
	}
	until := time.Now().Add(24 * time.Hour)
	row := effectBase(caster, TheDonsFavorCardID, "DON")
	row.TargetUserID = &target.DiscordID
	row.ExpiresAt = &until
	return tx.Create(&row).Error
}

func PlaceDice(tx *gorm.DB, user models.User, entry models.BetEntry, roll float64) (bool, error) {
	var count int64
	if err := tx.Model(&models.UserInventory{}).
		Where("user_id = ? AND card_id = ? AND target_entry_id = ? AND deleted_at IS NULL", user.ID, LoadedDiceCardID, entry.ID).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if err := ConsumeHeldCopy(tx, user.ID, user.GuildID, LoadedDiceCardID); err != nil {
		return false, err
	}
	row := effectBase(user, LoadedDiceCardID, "LDD")
	row.TargetBetID = &entry.BetID
	row.TargetEntryID = &entry.ID
	row.BetAmount = roll
	return true, tx.Create(&row).Error
}

func PlaceFix(tx *gorm.DB, user models.User, entry models.BetEntry) (bool, error) {
	var count int64
	if err := tx.Model(&models.UserInventory{}).
		Where("user_id = ? AND card_id = ? AND target_bet_id = ? AND deleted_at IS NULL", user.ID, TheFixIsInCardID, entry.BetID).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if err := ConsumeHeldCopy(tx, user.ID, user.GuildID, TheFixIsInCardID); err != nil {
		return false, err
	}
	row := effectBase(user, TheFixIsInCardID, "FIX")
	row.TargetBetID = &entry.BetID
	row.TargetEntryID = &entry.ID
	return true, tx.Create(&row).Error
}

func PlaceKnuckles(tx *gorm.DB, user models.User, entry models.BetEntry, targetDiscordID string) (bool, error) {
	var count int64
	if err := tx.Model(&models.UserInventory{}).
		Where("user_id = ? AND card_id = ? AND target_entry_id = ? AND deleted_at IS NULL", user.ID, BrassKnucklesCardID, entry.ID).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	var both int64
	if err := tx.Model(&models.BetEntry{}).
		Where("bet_id = ? AND user_id = ? AND option <> ? AND deleted_at IS NULL", entry.BetID, user.ID, entry.Option).
		Count(&both).Error; err != nil {
		return false, err
	}
	if both > 0 {
		return false, nil
	}
	if err := ConsumeHeldCopy(tx, user.ID, user.GuildID, BrassKnucklesCardID); err != nil {
		return false, err
	}
	row := effectBase(user, BrassKnucklesCardID, "BKN")
	row.TargetBetID = &entry.BetID
	row.TargetEntryID = &entry.ID
	row.TargetUserID = &targetDiscordID
	return true, tx.Create(&row).Error
}

func ShaveEntry(tx *gorm.DB, user models.User, entry models.BetEntry) error {
	if entry.Spread == nil {
		return fmt.Errorf("entry has no spread")
	}
	shifted := ShiftSpreadInFavor(*entry.Spread, entry.Option)
	if err := tx.Model(&models.BetEntry{}).Where("id = ?", entry.ID).Update("spread", shifted).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.ParlayEntry{}).
		Where("bet_id = ? AND selected_option = ? AND deleted_at IS NULL AND parlay_id IN (?)",
			entry.BetID, entry.Option,
			tx.Model(&models.Parlay{}).Select("id").Where("user_id = ? AND guild_id = ?", user.ID, user.GuildID)).
		Update("spread", shifted).Error; err != nil {
		return err
	}
	return ConsumeHeldCopy(tx, user.ID, user.GuildID, PointShaverCardID)
}

func OpenLineEntry(tx *gorm.DB, user models.User, entry models.BetEntry, opening *float64) (bool, error) {
	if opening == nil {
		return false, nil
	}
	if err := tx.Model(&models.BetEntry{}).Where("id = ?", entry.ID).Update("spread", *opening).Error; err != nil {
		return false, err
	}
	if err := tx.Model(&models.ParlayEntry{}).
		Where("bet_id = ? AND selected_option = ? AND deleted_at IS NULL AND parlay_id IN (?)",
			entry.BetID, entry.Option,
			tx.Model(&models.Parlay{}).Select("id").Where("user_id = ? AND guild_id = ?", user.ID, user.GuildID)).
		Update("spread", *opening).Error; err != nil {
		return false, err
	}
	if err := ConsumeHeldCopy(tx, user.ID, user.GuildID, BackroomWiretapCardID); err != nil {
		return false, err
	}
	return true, nil
}

func RefundEntry(tx *gorm.DB, user models.User, entry models.BetEntry) error {
	if err := tx.Model(&models.User{}).Where("id = ?", user.ID).
		UpdateColumn("points", gorm.Expr("points + ?", entry.Amount)).Error; err != nil {
		return err
	}
	if err := tx.Delete(&models.BetEntry{}, entry.ID).Error; err != nil {
		return err
	}
	if err := tx.Where("target_entry_id = ? AND deleted_at IS NULL", entry.ID).Delete(&models.UserInventory{}).Error; err != nil {
		return err
	}
	return ConsumeHeldCopy(tx, user.ID, user.GuildID, BackroomWiretapCardID)
}

func PlaceSpeakeasy(tx *gorm.DB, user models.User, parlay models.Parlay) (bool, error) {
	var legs int64
	if err := tx.Model(&models.ParlayEntry{}).Where("parlay_id = ? AND deleted_at IS NULL", parlay.ID).Count(&legs).Error; err != nil {
		return false, err
	}
	if legs < 4 {
		return false, nil
	}
	if parlay.Status != "pending" && parlay.Status != "partial" {
		return false, nil
	}
	var losses int64
	if err := tx.Model(&models.ParlayEntry{}).
		Where("parlay_id = ? AND deleted_at IS NULL AND won = ?", parlay.ID, false).
		Count(&losses).Error; err != nil {
		return false, err
	}
	if losses > 0 {
		return false, nil
	}
	var existing int64
	if err := tx.Model(&models.UserInventory{}).
		Where("card_id = ? AND target_parlay_id = ? AND deleted_at IS NULL", SpeakeasyParlayCardID, parlay.ID).
		Count(&existing).Error; err != nil {
		return false, err
	}
	if existing > 0 {
		return false, nil
	}
	if err := ConsumeHeldCopy(tx, user.ID, user.GuildID, SpeakeasyParlayCardID); err != nil {
		return false, err
	}
	row := effectBase(user, SpeakeasyParlayCardID, "SPK")
	row.TargetParlayID = &parlay.ID
	return true, tx.Create(&row).Error
}

type swappedCard struct {
	Name string
}

func LeveragedBuyoutSwap(tx *gorm.DB, caster models.User, target models.User) (given, taken string, ok bool, err error) {
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&caster, caster.ID).Error; err != nil {
		return "", "", false, err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, target.ID).Error; err != nil {
		return "", "", false, err
	}
	give, err := randomUnattached(tx, caster.ID, caster.GuildID, []string{"Common"})
	if err != nil || give == nil {
		return "", "", false, err
	}
	take, err := randomUnattached(tx, target.ID, target.GuildID, []string{"Rare", "Epic"})
	if err != nil || take == nil {
		return "", "", false, err
	}
	if err := tx.Model(&models.UserInventory{}).Where("id = ?", give.ID).Update("user_id", target.ID).Error; err != nil {
		return "", "", false, err
	}
	if err := tx.Model(&models.UserInventory{}).Where("id = ?", take.ID).Update("user_id", caster.ID).Error; err != nil {
		return "", "", false, err
	}
	if err := ConsumeHeldCopy(tx, caster.ID, caster.GuildID, LeveragedBuyoutCardID); err != nil {
		return "", "", false, err
	}
	return give.Name, take.Name, true, nil
}

type namedInventory struct {
	ID   uint
	Name string
}

func randomUnattached(tx *gorm.DB, userID uint, guildID string, rarities []string) (*namedInventory, error) {
	var rows []namedInventory
	err := tx.Table("user_inventories").
		Select("user_inventories.id, cards.name").
		Joins("JOIN cards ON cards.id = user_inventories.card_id").
		Joins("JOIN card_rarities ON card_rarities.id = cards.rarity_id").
		Where(`user_inventories.user_id = ? AND user_inventories.guild_id = ? AND user_inventories.deleted_at IS NULL
			AND user_inventories.target_bet_id IS NULL AND user_inventories.target_entry_id IS NULL
			AND user_inventories.target_parlay_id IS NULL AND user_inventories.target_user_id IS NULL
			AND user_inventories.expires_at IS NULL AND user_inventories.bet_amount = 0
			AND user_inventories.times_applied = 0
			AND card_rarities.name IN ?`, userID, guildID, rarities).
		Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	pick := rows[rand.Intn(len(rows))]
	return &pick, nil
}
