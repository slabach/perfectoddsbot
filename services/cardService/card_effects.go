package cardService

import (
	"math/rand"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"time"

	"gorm.io/gorm"
)

type CardConsumer func(db *gorm.DB, user models.User, cardID uint) error

type AntiAntiBetWinner struct {
	DiscordID string
	Payout    float64
}

type VampireWinner struct {
	DiscordID string
	Payout    float64
}

type LoversWinner struct {
	DiscordID string
	Payout    float64
}

type DevilDiverted struct {
	DiscordID string
	Diverted  float64
}

type EmperorDiverted struct {
	DiscordID string
	Diverted  float64
}

func ApplyUnoReverseIfApplicable(db *gorm.DB, user models.User, betID uint, originalIsWin bool) (bool, bool, error) {
	var inventory models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND target_bet_id = ?", user.ID, user.GuildID, cards.UnoReverseCardID, betID).
		First(&inventory).Error

	if err == gorm.ErrRecordNotFound {
		return false, originalIsWin, nil
	}
	if err != nil {
		return false, originalIsWin, err
	}

	if err := db.Delete(&inventory).Error; err != nil {
		return false, originalIsWin, err
	}

	return true, !originalIsWin, nil
}

func ApplyAntiAntiBetIfApplicable(db *gorm.DB, bettorUser models.User, isWin bool) (totalPayout float64, winners []AntiAntiBetWinner, losers []AntiAntiBetWinner, applied bool, err error) {
	var userCards []models.UserInventory
	err = db.Where("guild_id = ? AND card_id = ? AND target_user_id = ?", bettorUser.GuildID, cards.AntiAntiBetCardID, bettorUser.DiscordID).
		Find(&userCards).Error

	if err != nil {
		return 0, nil, nil, false, err
	}

	if len(userCards) == 0 {
		return 0, nil, nil, false, nil
	}

	totalPayout = 0.0
	applied = true
	winners = []AntiAntiBetWinner{}
	losers = []AntiAntiBetWinner{}

	for _, card := range userCards {
		if isWin {
			var cardHolder models.User
			if err := db.First(&cardHolder, card.UserID).Error; err != nil {
				db.Delete(&card)
				continue
			}

			losers = append(losers, AntiAntiBetWinner{
				DiscordID: cardHolder.DiscordID,
				Payout:    card.BetAmount,
			})

			if err := db.Delete(&card).Error; err != nil {
				return totalPayout, winners, losers, applied, err
			}
		} else {
			payout := common.CalculateSimplePayout(card.BetAmount)

			var cardHolder models.User
			if err := db.First(&cardHolder, card.UserID).Error; err != nil {
				db.Delete(&card)
				continue
			}

			cardHolder.Points += payout
			if err := db.Model(&cardHolder).UpdateColumn("points", gorm.Expr("points + ?", payout)).Error; err != nil {
				return totalPayout, winners, losers, applied, err
			}

			totalPayout += payout
			winners = append(winners, AntiAntiBetWinner{
				DiscordID: cardHolder.DiscordID,
				Payout:    payout,
			})

			if err := db.Delete(&card).Error; err != nil {
				return totalPayout, winners, losers, applied, err
			}
		}
	}

	return totalPayout, winners, losers, applied, nil
}

func ApplyVampireIfApplicable(db *gorm.DB, guildID string, totalWinningPayouts float64, winnerDiscordIDs map[string]float64) (totalVampirePayout float64, winners []VampireWinner, applied bool, err error) {
	twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)
	var vampireCards []models.UserInventory
	err = db.Where("guild_id = ? AND card_id = ? AND created_at >= ?", guildID, cards.VampireCardID, twentyFourHoursAgo).
		Find(&vampireCards).Error

	if err != nil {
		return 0, nil, false, err
	}

	if len(vampireCards) == 0 {
		return 0, nil, false, nil
	}

	totalVampirePayout = 0.0
	applied = true
	winners = []VampireWinner{}

	for _, card := range vampireCards {
		var vampireHolder models.User
		if err := db.First(&vampireHolder, card.UserID).Error; err != nil {
			continue
		}

		vampireHolderWinnings := 0.0
		if winnerDiscordIDs != nil {
			vampireHolderWinnings = winnerDiscordIDs[vampireHolder.DiscordID]
		}
		totalOtherWinnings := totalWinningPayouts - vampireHolderWinnings

		if totalOtherWinnings < 0 {
			continue
		}
		if totalWinningPayouts > 0 && totalOtherWinnings == 0 {
			continue
		}

		vampirePayout := totalOtherWinnings * 0.05

		if vampirePayout > 500.0 {
			vampirePayout = 500.0
		}

		if err := db.Model(&vampireHolder).UpdateColumn("points", gorm.Expr("points + ?", vampirePayout)).Error; err != nil {
			return totalVampirePayout, winners, applied, err
		}

		totalVampirePayout += vampirePayout
		winners = append(winners, VampireWinner{
			DiscordID: vampireHolder.DiscordID,
			Payout:    vampirePayout,
		})
	}

	return totalVampirePayout, winners, applied, nil
}

func ApplyTheDevilIfApplicable(db *gorm.DB, guildID string, winnerDiscordIDs map[string]float64) (totalDiverted float64, diverted []DevilDiverted, applied bool, err error) {
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)
	var devilCards []models.UserInventory
	err = db.Where("guild_id = ? AND card_id = ? AND created_at >= ? AND deleted_at IS NULL", guildID, cards.TheDevilCardID, sevenDaysAgo).
		Find(&devilCards).Error

	if err != nil {
		return 0, nil, false, err
	}

	if len(devilCards) == 0 {
		return 0, nil, false, nil
	}

	devilCardHolders := make(map[string]uint)
	for _, card := range devilCards {
		var user models.User
		if err := db.First(&user, card.UserID).Error; err != nil {
			continue
		}
		devilCardHolders[user.DiscordID] = user.ID
	}

	if len(devilCardHolders) == 0 {
		return 0, nil, false, nil
	}

	totalDiverted = 0.0
	diverted = []DevilDiverted{}

	var guild models.Guild
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return 0, nil, false, err
	}

	for discordID, winnings := range winnerDiscordIDs {
		userID, hasDevilCard := devilCardHolders[discordID]
		if !hasDevilCard || winnings <= 0 {
			continue
		}

		divertedAmount := winnings * 0.20
		totalDiverted += divertedAmount
		winnerDiscordIDs[discordID] = winnings - divertedAmount

		if err := db.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("points", gorm.Expr("points - ?", divertedAmount)).Error; err != nil {
			return totalDiverted, diverted, totalDiverted > 0, err
		}

		diverted = append(diverted, DevilDiverted{
			DiscordID: discordID,
			Diverted:  divertedAmount,
		})
	}

	if totalDiverted > 0 {
		applied = true
		if err := db.Model(&guild).UpdateColumn("pool", gorm.Expr("pool + ?", totalDiverted)).Error; err != nil {
			return totalDiverted, diverted, applied, err
		}
	}

	return totalDiverted, diverted, applied, nil
}

func ApplyTheEmperorIfApplicable(db *gorm.DB, guildID string, winnerDiscordIDs map[string]float64) (totalDiverted float64, diverted []EmperorDiverted, applied bool, err error) {
	var guild models.Guild
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return 0, nil, false, err
	}

	now := time.Now()
	if guild.EmperorActiveUntil == nil || now.After(*guild.EmperorActiveUntil) {
		if guild.EmperorActiveUntil != nil || guild.EmperorHolderDiscordID != nil {
			_ = db.Model(&guild).Updates(map[string]interface{}{
				"emperor_active_until":      nil,
				"emperor_holder_discord_id": nil,
			})
		}
		return 0, nil, false, nil
	}
	if guild.EmperorHolderDiscordID == nil {
		return 0, nil, false, nil
	}

	holderDiscordID := *guild.EmperorHolderDiscordID
	totalDiverted = 0.0
	applied = true
	diverted = []EmperorDiverted{}

	for discordID, winnings := range winnerDiscordIDs {
		if discordID == holderDiscordID || winnings <= 0 {
			continue
		}
		divertedAmount := winnings * 0.10
		totalDiverted += divertedAmount
		winnerDiscordIDs[discordID] = winnings - divertedAmount

		var user models.User
		if err := db.Where("discord_id = ? AND guild_id = ?", discordID, guildID).First(&user).Error; err != nil {
			return totalDiverted, diverted, applied, err
		}
		if err := db.Model(&user).UpdateColumn("points", gorm.Expr("points - ?", divertedAmount)).Error; err != nil {
			return totalDiverted, diverted, applied, err
		}

		diverted = append(diverted, EmperorDiverted{
			DiscordID: discordID,
			Diverted:  divertedAmount,
		})
	}

	if totalDiverted > 0 {
		if err := db.Model(&guild).UpdateColumn("pool", gorm.Expr("pool + ?", totalDiverted)).Error; err != nil {
			return totalDiverted, diverted, applied, err
		}
	}

	return totalDiverted, diverted, applied, nil
}

func ApplyTheLoversIfApplicable(db *gorm.DB, guildID string, winnerDiscordIDs map[string]float64) (totalLoversPayout float64, winners []LoversWinner, applied bool, err error) {
	twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)
	var loversCards []models.UserInventory
	err = db.Where("guild_id = ? AND card_id = ? AND target_user_id IS NOT NULL AND deleted_at IS NULL", guildID, cards.TheLoversCardID).
		Find(&loversCards).Error

	if err != nil {
		return 0, nil, false, err
	}

	if len(loversCards) == 0 {
		return 0, nil, false, nil
	}

	totalLoversPayout = 0.0
	applied = true
	winners = []LoversWinner{}

	for _, card := range loversCards {
		if card.TargetUserID == nil {
			continue
		}

		if card.CreatedAt.Before(twentyFourHoursAgo) {
			if err := db.Delete(&card).Error; err != nil {
				return totalLoversPayout, winners, applied, err
			}
			continue
		}

		targetUserID := *card.TargetUserID
		targetWinnings, targetWon := winnerDiscordIDs[targetUserID]

		if !targetWon || targetWinnings <= 0 {
			continue
		}

		var loversHolder models.User
		if err := db.First(&loversHolder, card.UserID).Error; err != nil {
			continue
		}

		loversPayout := targetWinnings * 0.25

		if err := db.Model(&loversHolder).UpdateColumn("points", gorm.Expr("points + ?", loversPayout)).Error; err != nil {
			return totalLoversPayout, winners, applied, err
		}

		totalLoversPayout += loversPayout
		winners = append(winners, LoversWinner{
			DiscordID: loversHolder.DiscordID,
			Payout:    loversPayout,
		})
	}

	return totalLoversPayout, winners, applied, nil
}

func ApplyDoubleDownIfAvailable(db *gorm.DB, consumer CardConsumer, user models.User, originalPayout float64) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.DoubleDownCardID).
		Count(&count).Error

	if err != nil {
		return originalPayout, false, err
	}

	if count > 0 {
		if err := consumer(db, user, cards.DoubleDownCardID); err != nil {
			return originalPayout, false, err
		}
		return originalPayout * 2.0, true, nil
	}

	return originalPayout, false, nil
}

func ApplyEmotionalHedgeIfApplicable(db *gorm.DB, consumer CardConsumer, user models.User, bet models.Bet, userPick int, betAmount float64, scoreDiff int) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.EmotionalHedgeCardID).
		Count(&count).Error
	if err != nil {
		return 0, false, err
	}
	if count == 0 {
		return 0, false, nil
	}

	var guild models.Guild
	if err := db.Where("guild_id = ?", user.GuildID).First(&guild).Error; err != nil {
		return 0, false, err
	}
	if guild.SubscribedTeam == nil || *guild.SubscribedTeam == "" {
		return 0, false, nil
	}
	subscribedTeam := *guild.SubscribedTeam

	var userPickedTeamName string
	if userPick == 1 {
		userPickedTeamName = bet.Option1
	} else {
		userPickedTeamName = bet.Option2
	}

	userPickedTeamNameNormalized := common.GetSchoolName(userPickedTeamName)
	subscribedTeamNormalized := common.GetSchoolName(subscribedTeam)

	isBetOnSubscribedTeam := userPickedTeamNameNormalized == subscribedTeamNormalized

	if !isBetOnSubscribedTeam {
		isBetOnSubscribedTeam = (userPickedTeamName == subscribedTeam)
	}

	if !isBetOnSubscribedTeam {
		return 0, false, nil
	}

	if scoreDiff == 0 {
		if err := consumer(db, user, cards.EmotionalHedgeCardID); err != nil {
			return 0, false, err
		}
		return 0, true, nil
	}

	teamWonStraightUp := false
	if userPick == 1 {
		teamWonStraightUp = scoreDiff > 0
	} else {
		teamWonStraightUp = scoreDiff < 0
	}

	if !teamWonStraightUp {
		if err := consumer(db, user, cards.EmotionalHedgeCardID); err != nil {
			return 0, false, err
		}
		refund := betAmount * 0.5
		return refund, true, nil
	}

	if err := consumer(db, user, cards.EmotionalHedgeCardID); err != nil {
		return 0, false, err
	}
	return 0, true, nil
}

func ApplyBetInsuranceIfApplicable(db *gorm.DB, consumer CardConsumer, user models.User, betAmount float64, isWin bool) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.BetInsuranceCardID).
		Count(&count).Error

	if err != nil {
		return 0, false, err
	}

	if count > 0 {
		if err := consumer(db, user, cards.BetInsuranceCardID); err != nil {
			return 0, false, err
		}

		if !isWin {
			refund := betAmount * 0.25
			return refund, true, nil
		} else {
			return 0, true, nil
		}
	}

	return 0, false, nil
}

func ApplyGetOutOfJailIfApplicable(db *gorm.DB, consumer CardConsumer, user models.User, betAmount float64) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.GetOutOfJailCardID).
		Count(&count).Error

	if err != nil {
		return 0, false, err
	}

	if count > 0 {
		if err := consumer(db, user, cards.GetOutOfJailCardID); err != nil {
			return 0, false, err
		}

		return betAmount, true, nil
	}

	return 0, false, nil
}

func ApplyGamblerIfAvailable(db *gorm.DB, consumer CardConsumer, user models.User, originalPayout float64, isWin bool) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.GamblerCardID).
		Count(&count).Error

	if err != nil {
		return originalPayout, false, err
	}

	if count > 0 {
		if err := consumer(db, user, cards.GamblerCardID); err != nil {
			return originalPayout, false, err
		}

		if rand.Intn(2) == 0 {
			return originalPayout * 2.0, true, nil
		}

		return originalPayout, true, nil
	}

	return originalPayout, false, nil
}

func ApplyHomeFieldAdvantageIfApplicable(db *gorm.DB, user models.User, currentPayout float64) (float64, bool, error) {
	var inv models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL",
		user.ID, user.GuildID, cards.HomeFieldAdvantageCardID).First(&inv).Error
	if err != nil {
		return currentPayout, false, nil
	}
	if inv.ExpiresAt == nil {
		return currentPayout, false, nil
	}
	if time.Now().Before(*inv.ExpiresAt) {
		return currentPayout + 15, true, nil
	}
	_ = db.Delete(&inv).Error
	return currentPayout, false, nil
}

func ApplyRoughingTheKickerIfApplicable(db *gorm.DB, user models.User, currentPayout float64) (float64, bool, error) {
	var inv models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL",
		user.ID, user.GuildID, cards.RoughingTheKickerCardID).First(&inv).Error
	if err != nil {
		return currentPayout, false, nil
	}
	reducedPayout := currentPayout * 0.85
	if err := db.Delete(&inv).Error; err != nil {
		return currentPayout, false, err
	}
	return reducedPayout, true, nil
}

// ApplyHeismanCampaignIfApplicable reduces the winner's payout by 15% if they have Heisman Campaign
// in inventory. The card is consumed (soft-deleted) when applied. Stacks with other modifiers (e.g. Roughing the Kicker).
func ApplyHeismanCampaignIfApplicable(db *gorm.DB, user models.User, currentPayout float64) (float64, bool, error) {
	var inv models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL",
		user.ID, user.GuildID, cards.HeismanCampaignCardID).First(&inv).Error
	if err != nil {
		return currentPayout, false, nil
	}
	reducedPayout := currentPayout * 0.85
	if err := db.Delete(&inv).Error; err != nil {
		return currentPayout, false, err
	}
	return reducedPayout, true, nil
}
