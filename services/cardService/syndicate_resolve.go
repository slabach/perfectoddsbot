package cardService

import (
	"fmt"
	"time"

	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func entrySpread(entry models.BetEntry, bet models.Bet) (float64, bool) {
	if entry.Spread != nil {
		return *entry.Spread, true
	}
	if bet.Spread != nil {
		return *bet.Spread, true
	}
	return 0, false
}

// ApplyLoadedDice multiplies one entry's odds payout when that entry has a valid roll.
// The row is consumed even when the roll is ignored, so it cannot apply twice.
func ApplyLoadedDice(db *gorm.DB, user models.User, entryID uint, payout float64) (float64, bool, error) {
	var row models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND target_entry_id = ? AND deleted_at IS NULL",
		user.ID, user.GuildID, cards.LoadedDiceCardID, entryID).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return payout, false, nil
	}
	if err != nil {
		return payout, false, err
	}
	if err := db.Delete(&row).Error; err != nil {
		return payout, false, err
	}
	if !cards.ValidLoadedDice(row.BetAmount) {
		return payout, false, nil
	}
	return payout * row.BetAmount, true, nil
}

// ApplyFixIsIn refunds a spread entry that loses by more than 0 and at most 3.
// The row is consumed only when it pushes, so a wider miss leaves it for... no:
// one bet resolves once, so consume on any resolution of that entry.
func ApplyFixIsIn(db *gorm.DB, user models.User, entry models.BetEntry, bet models.Bet, scoreDiff int) (bool, error) {
	if bet.Spread == nil {
		return false, nil
	}
	var row models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND target_entry_id = ? AND deleted_at IS NULL",
		user.ID, user.GuildID, cards.TheFixIsInCardID, entry.ID).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	spread, ok := entrySpread(entry, bet)
	if !ok {
		_ = db.Delete(&row)
		return false, nil
	}
	miss := cards.ATSMiss(entry.Option, float64(scoreDiff), spread)
	push := cards.IsFixPush(miss)
	if err := db.Delete(&row).Error; err != nil {
		return false, err
	}
	return push, nil
}

// ConsumeFixForBet drops Fix rows when an admin resolves the bet and there is no score.
func ConsumeFixForBet(db *gorm.DB, betID uint) error {
	return db.Where("card_id = ? AND target_bet_id = ? AND deleted_at IS NULL", cards.TheFixIsInCardID, betID).
		Delete(&models.UserInventory{}).Error
}

// ApplyBrassKnuckles steals 5% of the target's opposite-side stake when the attached entry wins.
// A loss or a push consumes the row and pays nothing.
func ApplyBrassKnuckles(db *gorm.DB, user models.User, entry models.BetEntry, won bool) (float64, error) {
	var row models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND target_entry_id = ? AND deleted_at IS NULL",
		user.ID, user.GuildID, cards.BrassKnucklesCardID, entry.ID).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if err := db.Delete(&row).Error; err != nil {
		return 0, err
	}
	if !won || row.TargetUserID == nil {
		return 0, nil
	}
	var target models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", *row.TargetUserID, user.GuildID).
		First(&target).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	label, err := cardsBlock(db, target.ID, user.GuildID)
	if err != nil {
		return 0, err
	}
	if label != "" {
		return 0, nil
	}
	var opposite int64
	if err := db.Model(&models.BetEntry{}).
		Where("bet_id = ? AND user_id = ? AND option <> ? AND deleted_at IS NULL", entry.BetID, target.ID, entry.Option).
		Select("COALESCE(SUM(amount), 0)").Scan(&opposite).Error; err != nil {
		return 0, err
	}
	steal := float64(opposite) * cards.BrassKnucklesRate
	if steal > target.Points {
		steal = target.Points
	}
	if steal <= 0 {
		return 0, nil
	}
	if err := db.Model(&target).UpdateColumn("points", gorm.Expr("points - ?", steal)).Error; err != nil {
		return 0, err
	}
	return steal, nil
}

func cardsBlock(db *gorm.DB, userDBID uint, guildID string) (string, error) {
	return blockLabel(db, userDBID, guildID)
}

func blockLabel(db *gorm.DB, userDBID uint, guildID string) (string, error) {
	books, err := cards.ProtectedByCookedBooks(db, userDBID, guildID)
	if err != nil {
		return "", err
	}
	if books {
		return "Cooked Books", nil
	}
	moon, err := cards.CheckAndConsumeMoon(db, userDBID, guildID)
	if err != nil {
		return "", err
	}
	if moon {
		return "The Moon", nil
	}
	blocked, protection, err := cards.CheckAndConsumeShieldOrRedshirt(db, userDBID, guildID)
	if err != nil || !blocked {
		return "", err
	}
	return protection, nil
}

type winTax struct {
	Payout    float64
	DonCut    float64
	RacketCut float64
}

// ApplyStraightWinTaxes skims the Don and then a Protection Racket from a straight-bet win.
func ApplyStraightWinTaxes(db *gorm.DB, user models.User, stake, payout float64) (winTax, error) {
	don, err := activeDonAgainst(db, user)
	if err != nil {
		return winTax{Payout: payout}, err
	}
	racket, err := activeRacketAgainst(db, user)
	if err != nil {
		return winTax{Payout: payout}, err
	}
	remaining, donCut, racketCut := cards.WinSkim(payout, stake, don != nil, racket != nil)
	if racket != nil {
		if err := db.Delete(racket).Error; err != nil {
			return winTax{}, err
		}
		if racketCut > 0 {
			if err := creditDiscord(db, user.GuildID, racket.UserID, racketCut); err != nil {
				return winTax{}, err
			}
		}
	}
	if don != nil && donCut > 0 {
		if err := creditDiscord(db, user.GuildID, don.UserID, donCut); err != nil {
			return winTax{}, err
		}
	}
	return winTax{Payout: remaining, DonCut: donCut, RacketCut: racketCut}, nil
}

// ApplyDonWinTax skims only the Don. Parlays use this so a racket is not consumed.
func ApplyDonWinTax(db *gorm.DB, user models.User, stake, payout float64) (float64, float64, error) {
	don, err := activeDonAgainst(db, user)
	if err != nil || don == nil {
		return payout, 0, err
	}
	remaining, donCut, _ := cards.WinSkim(payout, stake, true, false)
	if donCut > 0 {
		if err := creditDiscord(db, user.GuildID, don.UserID, donCut); err != nil {
			return payout, 0, err
		}
	}
	return remaining, donCut, nil
}

// ApplyRacketOnLoss burns the target's next-bet racket without paying.
func ApplyRacketOnLoss(db *gorm.DB, user models.User) error {
	racket, err := activeRacketAgainst(db, user)
	if err != nil || racket == nil {
		return err
	}
	return db.Delete(racket).Error
}

// ApplyDonLossBill charges each partner 25% of the stake the caster actually lost.
// The favor row lives on the caster (user_id) and names the partner in target_user_id.
// A zero bill does nothing. Cooked Books, The Moon, Redshirt, and Shield on the partner block that favor.
func ApplyDonLossBill(db *gorm.DB, user models.User, unreimbursed float64) (float64, error) {
	if unreimbursed <= 0 {
		return 0, nil
	}
	var favors []models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL AND expires_at IS NOT NULL AND expires_at > ?",
		user.ID, user.GuildID, cards.TheDonsFavorCardID, time.Now()).Find(&favors).Error
	if err != nil || len(favors) == 0 {
		return 0, err
	}
	total := 0.0
	for _, favor := range favors {
		if favor.TargetUserID == nil {
			continue
		}
		var partner models.User
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", *favor.TargetUserID, user.GuildID).
			First(&partner).Error; err != nil {
			continue
		}
		label, err := blockLabel(db, partner.ID, user.GuildID)
		if err != nil {
			return total, err
		}
		if label != "" {
			continue
		}
		bill := cards.DonLossBill(unreimbursed, partner.Points)
		if bill <= 0 {
			continue
		}
		if err := db.Model(&partner).UpdateColumn("points", gorm.Expr("points - ?", bill)).Error; err != nil {
			return total, err
		}
		if err := db.Model(&models.User{}).Where("id = ?", user.ID).UpdateColumn("points", gorm.Expr("points + ?", bill)).Error; err != nil {
			return total, err
		}
		total += bill
	}
	return total, nil
}

func activeDonAgainst(db *gorm.DB, target models.User) (*models.UserInventory, error) {
	books, err := cards.ProtectedByCookedBooks(db, target.ID, target.GuildID)
	if err != nil || books {
		return nil, err
	}
	var row models.UserInventory
	err = db.Where("guild_id = ? AND card_id = ? AND deleted_at IS NULL AND target_user_id = ? AND expires_at IS NOT NULL AND expires_at > ?",
		target.GuildID, cards.TheDonsFavorCardID, target.DiscordID, time.Now()).
		Order("created_at ASC").First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func activeRacketAgainst(db *gorm.DB, target models.User) (*models.UserInventory, error) {
	var row models.UserInventory
	err := db.Where("guild_id = ? AND card_id = ? AND deleted_at IS NULL AND target_user_id = ?",
		target.GuildID, cards.ProtectionRacketCardID, target.DiscordID).
		Order("created_at ASC").First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	books, err := cards.ProtectedByCookedBooks(db, target.ID, target.GuildID)
	if err != nil {
		return nil, err
	}
	if books {
		if err := db.Delete(&row).Error; err != nil {
			return nil, err
		}
		return nil, nil
	}
	return &row, nil
}

func creditDiscord(db *gorm.DB, guildID string, userDBID uint, amount float64) error {
	if amount <= 0 {
		return nil
	}
	var holder models.User
	if err := db.First(&holder, userDBID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	if holder.GuildID != guildID {
		return nil
	}
	return db.Model(&holder).UpdateColumn("points", gorm.Expr("points + ?", amount)).Error
}

// ClearEntryAttachments removes effect rows pointed at entries that were deleted
// outside the normal resolver.
func ClearEntryAttachments(db *gorm.DB, entryIDs []uint) error {
	if len(entryIDs) == 0 {
		return nil
	}
	return db.Where("target_entry_id IN ? AND deleted_at IS NULL", entryIDs).Delete(&models.UserInventory{}).Error
}

// ClearBetAttachments removes effect rows for a bet that was settled or wiped.
func ClearBetAttachments(db *gorm.DB, betID uint) error {
	return db.Where("target_bet_id = ? AND deleted_at IS NULL", betID).Delete(&models.UserInventory{}).Error
}

// ParlayHasSpeakeasy reports whether a ticket is insured.
func ParlayHasSpeakeasy(db *gorm.DB, parlayID uint) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("card_id = ? AND target_parlay_id = ? AND deleted_at IS NULL", cards.SpeakeasyParlayCardID, parlayID).
		Count(&count).Error
	return count > 0, err
}

// ConsumeParlaySpeakeasy deletes the insurance row once the ticket settles.
func ConsumeParlaySpeakeasy(db *gorm.DB, parlayID uint) error {
	return db.Where("card_id = ? AND target_parlay_id = ? AND deleted_at IS NULL", cards.SpeakeasyParlayCardID, parlayID).
		Delete(&models.UserInventory{}).Error
}

// DivertBagman moves 15% of a draw or purchase to the holder. The returned
// pool share is what the pool should receive. A missing holder keeps 100% in the pool.
func DivertBagman(tx *gorm.DB, guild *models.Guild, paid float64) (poolShare float64, holderCut float64, err error) {
	poolShare = paid
	if guild.BagmanActiveUntil == nil || guild.BagmanHolderDiscordID == nil || !time.Now().Before(*guild.BagmanActiveUntil) {
		return poolShare, 0, nil
	}
	holderCut, poolShare = cards.BagmanSplit(paid)
	if holderCut <= 0 {
		return paid, 0, nil
	}
	var holder models.User
	err = tx.Where("discord_id = ? AND guild_id = ?", *guild.BagmanHolderDiscordID, guild.GuildID).First(&holder).Error
	if err == gorm.ErrRecordNotFound {
		return paid, 0, nil
	}
	if err != nil {
		return paid, 0, err
	}
	if err := tx.Model(&holder).UpdateColumn("points", gorm.Expr("points + ?", holderCut)).Error; err != nil {
		return paid, 0, err
	}
	return poolShare, holderCut, nil
}

// FinishSyndicateWin applies Don's Favor and Protection Racket to a straight win,
// then settles Brass Knuckles for that entry.
// knuckleSteal is not part of the bet payout. Callers add it to the winner's
// in-memory balance before saving, because a later Save would wipe an atomic credit.
func FinishSyndicateWin(db *gorm.DB, user models.User, entry models.BetEntry, payout float64) (float64, float64) {
	taxes, err := ApplyStraightWinTaxes(db, user, float64(entry.Amount), payout)
	if err != nil {
		return payout, 0
	}
	steal, _ := ApplyBrassKnuckles(db, user, entry, true)
	return taxes.Payout, steal
}

// FinishSyndicateLoss settles a straight loss. unreimbursed is the stake that was not refunded.
// A full refund passes 0 so the partner is not billed. The racket still burns because the bet resolved as a loss.
func FinishSyndicateLoss(db *gorm.DB, user models.User, entry models.BetEntry, unreimbursed float64) {
	_, _ = ApplyBrassKnuckles(db, user, entry, false)
	_ = ApplyRacketOnLoss(db, user)
	_, _ = ApplyDonLossBill(db, user, unreimbursed)
}

func BagmanNote(holderCut float64) string {
	if holderCut <= 0 {
		return ""
	}
	return fmt.Sprintf(" | Bagman's Cut: %.1f", holderCut)
}
