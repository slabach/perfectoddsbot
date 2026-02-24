package cards

import (
	"fmt"
	"math"
	"math/rand"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func handleDud(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "Nothing happened. You drew a blank card.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handlePenny(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You found a penny! +1 Point.",
		PointsDelta: 1,
		PoolDelta:   0,
	}, nil
}

func handlePapercut(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	deductAmount := 10.0
	if user.Points < deductAmount {
		deductAmount = user.Points
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("You cut your finger drawing the card. -%.0f Points.", deductAmount),
		PointsDelta: -deductAmount,
		PoolDelta:   0,
	}, nil
}

func handleVendingMachine(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You found loose change in the return slot. +25 Points.",
		PointsDelta: 25,
		PoolDelta:   0,
	}, nil
}

func handleGrail(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	guild, err := guildService.GetGuildInfo(s, db, guildID, "")
	if err != nil {
		return nil, err
	}

	poolWin := guild.Pool * 0.25

	return &models.CardResult{
		Message:     "You discovered the Holy Grail! You won 25% of the pool!",
		PointsDelta: poolWin,
		PoolDelta:   -poolWin,
	}, nil
}

func handleJackpot(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	guild, err := guildService.GetGuildInfo(s, db, guildID, "")
	if err != nil {
		return nil, err
	}

	poolWin := guild.Pool * 0.5

	return &models.CardResult{
		Message:     ":rotating_light: You discovered the JACKPOT! You won 50% of the pool! :rotating_light:",
		PointsDelta: poolWin,
		PoolDelta:   -poolWin,
	}, nil
}

func handlePettyTheft(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Petty Theft requires you to select a target!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecutePickpocketSteal(db *gorm.DB, userID string, targetUserID string, guildID string, amount float64) (*models.CardResult, error) {
	var user models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", userID, guildID).
		First(&user).Error; err != nil {
		return nil, err
	}
	userMention := "<@" + userID + ">"

	var targetUser models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
		First(&targetUser).Error; err != nil {
		return nil, err
	}

	targetMention := "<@" + targetUserID + ">"

	moonRedirected, err := CheckAndConsumeMoon(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if moonRedirected {
		randomUserID, err := GetRandomUserForMoon(db, guildID, []uint{targetUser.ID, user.ID})
		if err != nil {
			targetID := targetUserID
			return &models.CardResult{
				Message:           fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. The theft fizzles out.", targetMention),
				PointsDelta:       0,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: 0,
			}, nil
		}

		var randomUser models.User
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", randomUserID, guildID).
			First(&randomUser).Error; err != nil {
			return nil, err
		}

		stealAmount := amount
		if stealAmount > randomUser.Points {
			stealAmount = randomUser.Points
		}

		user.Points += stealAmount
		randomUser.Points -= stealAmount
		if err := db.Save(&randomUser).Error; err != nil {
			return nil, err
		}
		if err := db.Save(&user).Error; err != nil {
			return nil, err
		}

		randomMention := "<@" + randomUserID + ">"
		targetID := randomUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's Moon illusion redirected the theft! %s stole %.0f points from %s instead!", targetMention, userMention, stealAmount, randomMention),
			PointsDelta:       stealAmount,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: -stealAmount,
		}, nil
	}

	blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if blocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's %s blocked the theft attempt!", targetMention, protectionName(blockedByRedshirt)),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: 0,
		}, nil
	}

	stealAmount := amount

	if stealAmount > targetUser.Points {
		stealAmount = targetUser.Points
	}

	user.Points += stealAmount
	targetUser.Points -= stealAmount

	if err := db.Save(&user).Error; err != nil {
		return nil, err
	}
	if err := db.Save(&targetUser).Error; err != nil {
		return nil, err
	}

	var bountyCards []models.UserInventory
	if err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL", targetUser.ID, guildID, BountyHunterCardID).
		Find(&bountyCards).Error; err != nil {
		return nil, err
	}

	bountyCount := len(bountyCards)
	bountyReward := 0.0
	bountyMessage := ""
	targetID := targetUserID

	if bountyCount > 0 {
		var guild models.Guild
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			First(&guild).Error; err != nil {
			return nil, err
		}

		totalBountyReward := float64(bountyCount) * 100.0

		if guild.Pool >= totalBountyReward {
			bountyReward = totalBountyReward
		} else {
			bountyReward = guild.Pool
		}

		for _, bountyCard := range bountyCards {
			if err := db.Delete(&bountyCard).Error; err != nil {
				return nil, err
			}
		}

		if bountyCount == 1 {
			bountyMessage = fmt.Sprintf(" %s also claimed 1 bounty (+%.0f points from pool)!", userMention, bountyReward)
		} else {
			bountyMessage = fmt.Sprintf(" %s also claimed %d bounties (+%.0f points from pool)!", userMention, bountyCount, bountyReward)
		}
	}

	return &models.CardResult{
		Message:           fmt.Sprintf("%s successfully pickpocketed their target!%s", userMention, bountyMessage),
		PointsDelta:       stealAmount + bountyReward,
		PoolDelta:         -bountyReward,
		TargetUserID:      &targetID,
		TargetPointsDelta: -stealAmount,
	}, nil
}

func handleNilFee(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	deductAmount := 50.0
	if user.Points < deductAmount {
		deductAmount = user.Points
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("You paid %.0f points to the pool to retain the kicker.", deductAmount),
		PointsDelta: -deductAmount,
		PoolDelta:   deductAmount,
	}, nil
}

func handleSmallRebate(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	var guild models.Guild
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return nil, err
	}

	var refundAmount float64
	switch user.CardDrawCount {
	case 1:
		refundAmount = guild.CardDrawCost
	case 2:
		refundAmount = guild.CardDrawCost * 10
	default:
		refundAmount = guild.CardDrawCost * 50
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("You got a rebate! Refunded %.0f points (the cost of this card).", refundAmount),
		PointsDelta: refundAmount,
		PoolDelta:   -refundAmount,
	}, nil
}

func handleTipJar(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		var userAbove models.User
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ? AND (points > ? OR (points = ? AND id < ?))", guildID, user.Points, user.Points, user.ID).
			Order("points DESC, id ASC").
			First(&userAbove)

		if query.Error != nil || userAbove.ID == 0 {
			result = &models.CardResult{
				Message:     "You're at the top of the leaderboard! No one to tip you. The card fizzles out.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		transferAmount := 10.0
		if userAbove.Points < transferAmount {
			transferAmount = userAbove.Points
		}

		user.Points += transferAmount
		userAbove.Points -= transferAmount

		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		if err := tx.Save(&userAbove).Error; err != nil {
			return err
		}

		targetID := userAbove.DiscordID
		result = &models.CardResult{
			Message:           fmt.Sprintf("You shook the tip jar! The person above you gave you %.1f points.", transferAmount),
			PointsDelta:       transferAmount,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: -transferAmount,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleNeutralSiteGame(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		// person with the smallest points that are still greater than the drawer's
		var userAbove models.User
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ? AND points > ?", guildID, user.Points).
			Order("points ASC").
			First(&userAbove)

		if query.Error != nil || userAbove.ID == 0 {
			result = &models.CardResult{
				Message:     "You're at the top of the leaderboard! There's no one above you to share the neutral site with. The card fizzles out.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		gainAmount := 40.0
		targetID := userAbove.DiscordID
		result = &models.CardResult{
			Message:           "Neutral site game! You and the person directly above you on the leaderboard each gain 40 points.",
			PointsDelta:       gainAmount,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: gainAmount,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleHoleInPocket(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	lossAmount := user.Points * 0.05

	lossAmount = float64(int(lossAmount*10+0.5)) / 10.0

	if lossAmount > user.Points {
		lossAmount = user.Points
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("You found a hole in your pocket! Lost %.1f points (5%% of your total) to the Pool.", lossAmount),
		PointsDelta: -lossAmount,
		PoolDelta:   lossAmount,
	}, nil
}

func handlePiggyBank(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	gainAmount := user.Points * 0.05

	gainAmount = float64(int(gainAmount*10+0.5)) / 10.0

	return &models.CardResult{
		Message:     fmt.Sprintf("You broke open your piggy bank! Gained %.1f points (5%% of your total) from the void.", gainAmount),
		PointsDelta: gainAmount,
		PoolDelta:   0,
	}, nil
}

func handleParticipationTrophy(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You received a participation trophy! +1 Point and a pat on the back.",
		PointsDelta: 1,
		PoolDelta:   0,
	}, nil
}

func handleTimeout(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	spareKeyBlocked, err := checkAndConsumeSpareKey(db, user.ID, guildID)
	if err != nil {
		return nil, err
	}
	if spareKeyBlocked {
		return &models.CardResult{
			Message:     fmt.Sprintf("<@%s>'s Spare Key blocked the Timeout! The card fizzles out.", userID),
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	moonRedirected, err := CheckAndConsumeMoon(db, user.ID, guildID)
	if err != nil {
		return nil, err
	}
	if moonRedirected {
		randomUserID, err := GetRandomUserForMoon(db, guildID, []uint{user.ID})
		if err != nil {
			spareKeyBlocked, err := checkAndConsumeSpareKey(db, user.ID, guildID)
			if err != nil {
				return nil, err
			}
			if spareKeyBlocked {
				return &models.CardResult{
					Message:     fmt.Sprintf("<@%s>'s Moon illusion tried to redirect, but no eligible users found. Spare Key blocked the Timeout instead!", userID),
					PointsDelta: 0,
					PoolDelta:   0,
				}, nil
			}
			shieldBlocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, user.ID, guildID)
			if err != nil {
				return nil, err
			}
			if shieldBlocked {
				return &models.CardResult{
					Message:     fmt.Sprintf("<@%s>'s Moon illusion tried to redirect, but no eligible users found. %s blocked the Timeout instead!", userID, protectionName(blockedByRedshirt)),
					PointsDelta: 0,
					PoolDelta:   0,
				}, nil
			}
			return &models.CardResult{
				Message:     fmt.Sprintf("<@%s>'s Moon illusion tried to redirect, but no eligible users found. The card fizzles out.", userID),
				PointsDelta: 0,
				PoolDelta:   0,
			}, nil
		}

		var randomUser models.User
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", randomUserID, guildID).
			First(&randomUser).Error; err != nil {
			return nil, err
		}

		lockoutUntil := time.Now().Add(2 * time.Hour)
		randomUser.CardDrawTimeoutUntil = &lockoutUntil
		if err := db.Save(&randomUser).Error; err != nil {
			return nil, err
		}

		randomMention := "<@" + randomUserID + ">"
		return &models.CardResult{
			Message:           fmt.Sprintf("<@%s>'s Moon illusion redirected the Timeout! %s cannot buy any new cards for 2 hours instead!", userID, randomMention),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &randomUserID,
			TargetPointsDelta: 0,
		}, nil
	}

	shieldBlocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, user.ID, guildID)
	if err != nil {
		return nil, err
	}
	if shieldBlocked {
		return &models.CardResult{
			Message:     fmt.Sprintf("<@%s>'s %s blocked the Timeout! The card fizzles out.", userID, protectionName(blockedByRedshirt)),
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	timeoutUntil := time.Now().Add(2 * time.Hour)
	user.CardDrawTimeoutUntil = &timeoutUntil

	if err := db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("<@%s> has been timed out! They cannot buy another card for 2 hours.", userID),
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleBadInvestment(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	deductAmount := 50.0
	if user.Points < deductAmount {
		deductAmount = user.Points
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("Your bad investment cost you dearly. -%.0f Points.", deductAmount),
		PointsDelta: -deductAmount,
		PoolDelta:   0,
	}, nil
}

func handleFoundWallet(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You found a wallet on the ground! +50 Points.",
		PointsDelta: 50,
		PoolDelta:   0,
	}, nil
}

func handleCharityCase(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ? and deleted_at is null", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other players found. The charity has no one to help.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	userPosition := 0
	for i, u := range allUsers {
		if u.ID == user.ID {
			userPosition = i + 1
			break
		}
	}

	if userPosition == 0 {
		return &models.CardResult{
			Message:     "Could not determine your position. The charity passed you by.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	totalPlayers := len(allUsers)
	bottom50PercentThreshold := totalPlayers / 2
	if totalPlayers%2 != 0 {
		bottom50PercentThreshold = (totalPlayers + 1) / 2
	}

	isBottom50Percent := userPosition > bottom50PercentThreshold

	if isBottom50Percent {
		return &models.CardResult{
			Message:     "Charity found you! You're in the bottom 50% of players. +75 Points.",
			PointsDelta: 75,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:     "You're doing too well for charity. Nothing happens.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleTaxSeason(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ? and deleted_at is null", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other players found. Tax season skipped you.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	userPosition := 0
	for i, u := range allUsers {
		if u.ID == user.ID {
			userPosition = i + 1
			break
		}
	}

	if userPosition == 0 {
		return &models.CardResult{
			Message:     "Could not determine your position. Tax season passed you by.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	totalPlayers := len(allUsers)
	top50PercentThreshold := totalPlayers / 2
	if totalPlayers%2 != 0 {
		top50PercentThreshold = (totalPlayers + 1) / 2
	}

	isTop50Percent := userPosition <= top50PercentThreshold

	if isTop50Percent {
		deductAmount := 75.0
		if user.Points < deductAmount {
			deductAmount = user.Points
		}

		return &models.CardResult{
			Message:     fmt.Sprintf("Tax season hit you hard! You're in the top 50%% of players. -%.0f Points.", deductAmount),
			PointsDelta: -deductAmount,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:     "You're not wealthy enough to be taxed. Nothing happens.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleLuckyHorseshoe(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You found a lucky horseshoe! Your next card purchase will cost half price.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleUnluckyCat(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "An unlucky cat crossed your path! Your next card purchase will cost double.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleExcessiveCelebration(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You cheered too hard! You lose 25 points and your next card draw cost will be doubled.",
		PointsDelta: -25,
		PoolDelta:   0,
	}, nil
}

func handleFullCourtPress(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ? AND deleted_at IS NULL", guildID, userID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other users in the server. Full Court Press fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	n := 3
	if len(allUsers) < n {
		n = len(allUsers)
	}
	perm := rand.Perm(len(allUsers))
	var tagged []string
	for i := 0; i < n; i++ {
		target := allUsers[perm[i]]
		inv := models.UserInventory{
			UserID:    target.ID,
			GuildID:   guildID,
			CardID:    FullCourtPressCardID,
			CardCode:  "FCP",
			ExpiresAt: nil,
		}
		if err := db.Create(&inv).Error; err != nil {
			return nil, err
		}
		tagged = append(tagged, "<@"+target.DiscordID+">")
	}

	msg := fmt.Sprintf("Full Court Press! %d random user(s) will have their next card draw cost doubled: %s", n, strings.Join(tagged, " "))
	return &models.CardResult{
		Message:     msg,
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleCoupon(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You found a coupon! Your next card purchase will be 25% off.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleFullRide(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "Full Ride! Your next card draw is free.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleTwoPointConversion(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	if rand.Intn(2) == 0 {
		return &models.CardResult{
			Message:     "Two-point conversion good! You gain 150 points!",
			PointsDelta: 150,
			PoolDelta:   0,
		}, nil
	}
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}
	lossAmount := 50.0
	if user.Points < lossAmount {
		lossAmount = user.Points
	}
	return &models.CardResult{
		Message:     fmt.Sprintf("Two-point conversion failed! You lose %.0f points to the pool.", lossAmount),
		PointsDelta: -lossAmount,
		PoolDelta:   lossAmount,
	}, nil
}

func handleRoughingTheKicker(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "Roughing the Kicker! You lose 15 points to the pool. Your next bet win payout will be cut by 15%.",
		PointsDelta: -15,
		PoolDelta:   15,
	}, nil
}

func handlePickpocketCommon(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ? and deleted_at is null", guildID, userID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other users found to pickpocket. The card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	randomIndex := rand.Intn(len(allUsers))
	targetUser := allUsers[randomIndex]

	stealAmount := 50.0
	if targetUser.Points < stealAmount {
		stealAmount = targetUser.Points
	}

	targetID := targetUser.DiscordID
	return &models.CardResult{
		Message:           fmt.Sprintf("You pickpocketed a random user and stole %.0f points!", stealAmount),
		PointsDelta:       stealAmount,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: -stealAmount,
	}, nil
}

func handleDroppedLoot(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ? and deleted_at is null", guildID, userID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other users found to give loot to. The card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	randomIndex := rand.Intn(len(allUsers))
	targetUser := allUsers[randomIndex]

	giveAmount := 50.0
	if user.Points < giveAmount {
		giveAmount = user.Points
	}

	targetID := targetUser.DiscordID
	return &models.CardResult{
		Message:           fmt.Sprintf("You dropped some loot! A random user received %.0f points.", giveAmount),
		PointsDelta:       -giveAmount,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: giveAmount,
	}, nil
}

func handleScrapingBy(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You're scraping by! +20 Points.",
		PointsDelta: 20,
		PoolDelta:   0,
	}, nil
}

func handleRust(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	deductAmount := 20.0
	if user.Points < deductAmount {
		deductAmount = user.Points
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("Rust has set in. -%.0f Points.", deductAmount),
		PointsDelta: -deductAmount,
		PoolDelta:   0,
	}, nil
}

func handleMinorGlitch(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	randomPoints := float64(rand.Intn(100) + 1)

	return &models.CardResult{
		Message:     fmt.Sprintf("A minor glitch occurred! You gained %.0f points.", randomPoints),
		PointsDelta: randomPoints,
		PoolDelta:   0,
	}, nil
}

func handleTheFool(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	randomAmount := float64(rand.Intn(100) + 1)
	isGain := rand.Intn(2) == 1

	var message string
	var pointsDelta float64

	if isGain {
		message = fmt.Sprintf("You took a leap of faith and it paid off! You gained %.0f points.", randomAmount)
		pointsDelta = randomAmount
	} else {
		deductAmount := randomAmount
		if user.Points < deductAmount {
			deductAmount = user.Points
		}
		message = fmt.Sprintf("You took a leap of faith and stumbled! You lost %.0f points.", deductAmount)
		pointsDelta = -deductAmount
	}

	return &models.CardResult{
		Message:     message,
		PointsDelta: pointsDelta,
		PoolDelta:   0,
	}, nil
}

func handleHighFive(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ?", guildID, userID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other users found to high-five. The card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	randomIndex := rand.Intn(len(allUsers))
	targetUser := allUsers[randomIndex]

	gainAmount := 10.0
	targetID := targetUser.DiscordID
	return &models.CardResult{
		Message:           fmt.Sprintf("You high-fived a random user! You both gained %.0f points!", gainAmount),
		PointsDelta:       gainAmount,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: gainAmount,
	}, nil
}

func handleRickRoll(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	deductAmount := 5.0
	if user.Points < deductAmount {
		deductAmount = user.Points
	}

	youtubeLink := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	return &models.CardResult{
		Message:     fmt.Sprintf("You got rick rolled! -%.0f Points.\n\n%s", deductAmount, youtubeLink),
		PointsDelta: -deductAmount,
		PoolDelta:   0,
	}, nil
}

func handlePocketSand(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	var guild models.Guild
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return nil, err
	}

	var refundAmount float64
	switch user.CardDrawCount {
	case 1:
		refundAmount = guild.CardDrawCost
	case 2:
		refundAmount = guild.CardDrawCost * 10
	default:
		refundAmount = guild.CardDrawCost * 50
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("It's very effective! Refunded %.0f points (the cost of this card).", refundAmount),
		PointsDelta: refundAmount,
		PoolDelta:   -refundAmount,
	}, nil
}

func handleShield(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "A shimmering barrier surrounds you. Your next negative effect against you will be blocked.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleRedshirt(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You gained a Redshirt! You're immune to negative effects for the next 2 hours.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleHomeFieldAdvantage(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}
	var inv models.UserInventory
	if err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL", user.ID, guildID, HomeFieldAdvantageCardID).First(&inv).Error; err != nil {
		return &models.CardResult{
			Message:     "You don't have a Home Field Advantage card to play. The card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := db.Model(&inv).Update("expires_at", expiresAt).Error; err != nil {
		return nil, err
	}
	return &models.CardResult{
		Message:     "Home Field Advantage is now active for 24 hours! You'll gain +15 points on every bet you win.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleTheMoon(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "The Moon's illusion surrounds you. The next negative effect played against you will target a random user instead.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleTheMagician(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "The Magician requires you to select a target user!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func handleTheWheelOfFortune(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "The Wheel of Fortune spins! Choose your fate...",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleTheSun(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}
		userMention := "<@" + userID + ">"

		var guild models.Guild
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			First(&guild).Error; err != nil {
			return err
		}

		var allUsers []models.User
		if err := tx.Where("guild_id = ? AND deleted_at IS NULL AND discord_id != ?", guildID, userID).Find(&allUsers).Error; err != nil {
			return err
		}

		gainAmount := 800.0
		totalPoolDrain := gainAmount * 2.0

		if len(allUsers) == 0 {
			if guild.Pool < gainAmount {
				totalPoolDrain = guild.Pool
				gainAmount = guild.Pool / 2.0
			}
			if guild.Pool < totalPoolDrain {
				totalPoolDrain = guild.Pool
				gainAmount = totalPoolDrain / 2.0
			}

			result = &models.CardResult{
				Message:     fmt.Sprintf("The Sun's radiance! %s gained %.0f points from the pool. No other players were found to share the blessing.", userMention, gainAmount),
				PointsDelta: gainAmount,
				PoolDelta:   -totalPoolDrain,
			}
			return nil
		}

		if guild.Pool < gainAmount {
			totalPoolDrain = guild.Pool
			gainAmount = totalPoolDrain / 2.0
		}

		if guild.Pool < totalPoolDrain {
			totalPoolDrain = guild.Pool
			gainAmount = totalPoolDrain / 2.0
		}

		randomIndex := rand.Intn(len(allUsers))
		randomUser := allUsers[randomIndex]

		randomMention := "<@" + randomUser.DiscordID + ">"
		result = &models.CardResult{
			Message:           fmt.Sprintf("The Sun's radiance! %s and %s both gained %.0f points from the pool!", userMention, randomMention, gainAmount),
			PointsDelta:       gainAmount,
			PoolDelta:         -totalPoolDrain,
			TargetUserID:      &randomUser.DiscordID,
			TargetPointsDelta: gainAmount,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleJudgement(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var guild models.Guild
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			First(&guild).Error; err != nil {
			return err
		}

		var allUsers []models.User
		if err := tx.Where("guild_id = ? AND deleted_at IS NULL", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
			return err
		}

		if len(allUsers) == 0 {
			result = &models.CardResult{
				Message:     "No players found in the server. Judgement has no one to judge.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		totalUsers := len(allUsers)
		top50PercentCount := totalUsers / 2
		bottom50PercentCount := totalUsers - top50PercentCount

		if top50PercentCount == 0 {
			result = &models.CardResult{
				Message:     "Not enough players for Judgement. Need at least 2 players.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		top50Users := allUsers[:top50PercentCount]
		bottom50Users := allUsers[top50PercentCount:]

		totalPointsToPool := 0.0
		var top50Details []string

		for i := range top50Users {
			var lockedUser models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&lockedUser, top50Users[i].ID).Error; err != nil {
				return err
			}

			pointsLoss := lockedUser.Points * 0.05
			if pointsLoss <= 0 {
				continue
			}

			username := common.GetUsernameWithDB(tx, s, guildID, lockedUser.DiscordID)
			moonRedirected, err := CheckAndConsumeMoon(tx, lockedUser.ID, guildID)
			if err != nil {
				return err
			}
			if moonRedirected {
				randomDiscordID, err := GetRandomUserForMoon(tx, guildID, []uint{lockedUser.ID})
				if err != nil {
					blocked, _, err := CheckAndConsumeShieldOrRedshirt(tx, lockedUser.ID, guildID)
					if err != nil {
						return err
					}
					if blocked {
						continue
					}
					continue
				}
				var randomUser models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("discord_id = ? AND guild_id = ?", randomDiscordID, guildID).First(&randomUser).Error; err != nil {
					return err
				}
				deduct := randomUser.Points * 0.05
				if deduct > randomUser.Points {
					deduct = randomUser.Points
				}
				if deduct > 0 {
					randomUser.Points -= deduct
					if randomUser.Points < 0 {
						randomUser.Points = 0
					}
					if err := tx.Save(&randomUser).Error; err != nil {
						return err
					}
					totalPointsToPool += deduct
					randomName := common.GetUsernameWithDB(tx, s, guildID, randomUser.DiscordID)
					top50Details = append(top50Details, fmt.Sprintf("%s's Moon → %s: -%.0f points", username, randomName, deduct))
				}
				continue
			}
			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedUser.ID, guildID)
			if err != nil {
				return err
			}
			if blocked {
				top50Details = append(top50Details, fmt.Sprintf("%s: %s blocked", username, protectionName(blockedByRedshirt)))
				continue
			}

			lockedUser.Points -= pointsLoss
			if lockedUser.Points < 0 {
				lockedUser.Points = 0
			}
			if err := tx.Save(&lockedUser).Error; err != nil {
				return err
			}
			totalPointsToPool += pointsLoss
			top50Details = append(top50Details, fmt.Sprintf("%s: -%.0f points", username, pointsLoss))
		}

		guild.Pool += totalPointsToPool

		poolDistribution := guild.Pool * 0.10
		gainPerBottomUser := 0.0
		if bottom50PercentCount > 0 {
			gainPerBottomUser = poolDistribution / float64(bottom50PercentCount)
		}

		var bottom50Details []string
		totalDistributed := 0.0

		for i := range bottom50Users {
			var lockedUser models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&lockedUser, bottom50Users[i].ID).Error; err != nil {
				return err
			}

			lockedUser.Points += gainPerBottomUser
			if err := tx.Save(&lockedUser).Error; err != nil {
				return err
			}
			totalDistributed += gainPerBottomUser
			username := common.GetUsernameWithDB(tx, s, guildID, lockedUser.DiscordID)
			bottom50Details = append(bottom50Details, fmt.Sprintf("%s: +%.0f points", username, gainPerBottomUser))
		}

		guild.Pool -= totalDistributed
		if guild.Pool < 0 {
			guild.Pool = 0
		}

		if err := tx.Save(&guild).Error; err != nil {
			return err
		}

		const maxLinesPerSection = 10
		message := "The final reckoning! Judgement has been passed:\n\n"
		message += fmt.Sprintf("**Top 50%% (lost 5%% of points to pool):**\n")
		if len(top50Details) > 0 {
			shown := 0
			for _, detail := range top50Details {
				if shown < maxLinesPerSection {
					message += detail + "\n"
					shown++
				}
			}
			if len(top50Details) > maxLinesPerSection {
				message += fmt.Sprintf("\n...and %d more.\n", len(top50Details)-maxLinesPerSection)
			}
		} else {
			message += "No players affected.\n"
		}
		message += fmt.Sprintf("\n**Bottom 50%% (gained 10%% of pool, split evenly):**\n")
		if len(bottom50Details) > 0 {
			shown := 0
			for _, detail := range bottom50Details {
				if shown < maxLinesPerSection {
					message += detail + "\n"
					shown++
				}
			}
			if len(bottom50Details) > maxLinesPerSection {
				message += fmt.Sprintf("\n...and %d more.\n", len(bottom50Details)-maxLinesPerSection)
			}
		} else {
			message += "No players affected.\n"
		}

		result = &models.CardResult{
			Message:     message,
			PointsDelta: 0,
			PoolDelta:   0,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleTheDevil(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "The Devil's temptation! You gain 1000 points immediately, but 20% of your future bet wins for the next 7 days will be diverted into the pool.",
		PointsDelta: 1000,
		PoolDelta:   0,
	}, nil
}

func handleSpareKey(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You found a spare key! This will block your next timeout or card-buying restriction.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleMajorGlitch(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var userCount int64
	if err := db.Model(&models.User{}).Where("guild_id = ?", guildID).Count(&userCount).Error; err != nil {
		return nil, err
	}

	if userCount == 0 {
		return &models.CardResult{
			Message:     "No users found in the server. The glitch fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	gainAmount := 100.0
	var updatedCount int64
	if err := db.Model(&models.User{}).
		Where("guild_id = ? AND discord_id != ?", guildID, userID).
		Count(&updatedCount).Error; err != nil {
		return nil, err
	}
	if updatedCount > 0 {
		if err := db.Model(&models.User{}).
			Where("guild_id = ? AND discord_id != ?", guildID, userID).
			Update("points", gorm.Expr("points + ?", gainAmount)).Error; err != nil {
			return nil, err
		}
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("A major glitch occurred! Everyone in the server gained %.0f points! (%d users affected)", gainAmount, int(updatedCount)+1),
		PointsDelta: gainAmount,
		PoolDelta:   0,
	}, nil
}

func handleDoubleDown(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "Your next winning bet payout will be doubled! (Does not apply to parlays)",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleEmotionalHedge(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "Emotional Hedge active! If your subscribed team loses straight up on your next bet, you get 50% refund.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleJester(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "The Jester requires you to select a target to mute!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecuteJesterMute(s *discordgo.Session, db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var targetUser models.User
	targetID := targetUserID
	targetMention := "<@" + targetUserID + ">"
	userMention := "<@" + userID + ">"
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
		First(&targetUser).Error; err == nil {
		moonRedirected, err := CheckAndConsumeMoon(db, targetUser.ID, guildID)
		if err != nil {
			return nil, err
		}
		if moonRedirected {
			randomUserID, err := GetRandomUserForMoon(db, guildID, []uint{targetUser.ID})
			if err != nil {
				blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, targetUser.ID, guildID)
				if err != nil {
					return nil, err
				}
				if blocked {
					return &models.CardResult{
						Message:           fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. %s blocked the Jester's curse instead!", targetMention, protectionName(blockedByRedshirt)),
						PointsDelta:       0,
						PoolDelta:         0,
						TargetUserID:      &targetID,
						TargetPointsDelta: 0,
					}, nil
				}
				spareKeyBlocked, err := checkAndConsumeSpareKey(db, targetUser.ID, guildID)
				if err != nil {
					return nil, err
				}
				if spareKeyBlocked {
					return &models.CardResult{
						Message:           fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. Spare Key blocked the Jester's curse instead!", targetMention),
						PointsDelta:       0,
						PoolDelta:         0,
						TargetUserID:      &targetID,
						TargetPointsDelta: 0,
					}, nil
				}
				return &models.CardResult{
					Message:           fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. The curse fizzles out.", targetMention),
					PointsDelta:       0,
					PoolDelta:         0,
					TargetUserID:      &targetID,
					TargetPointsDelta: 0,
				}, nil
			}

			timeoutUntil := time.Now().Add(15 * time.Minute)
			if err := s.GuildMemberTimeout(guildID, randomUserID, &timeoutUntil); err != nil {
				return &models.CardResult{
					Message:     fmt.Sprintf("%s failed to mute the redirected target! They might be an Admin or too powerful.", userMention),
					PointsDelta: 0,
					PoolDelta:   0,
				}, nil
			}

			randomMention := "<@" + randomUserID + ">"
			randomTargetID := randomUserID
			return &models.CardResult{
				Message:           fmt.Sprintf("%s's Moon illusion redirected the Jester's (%s) curse! %s was muted for 15 minutes instead!", targetMention, userMention, randomMention),
				PointsDelta:       0,
				PoolDelta:         0,
				TargetUserID:      &randomTargetID,
				TargetPointsDelta: 0,
			}, nil
		}

		blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, targetUser.ID, guildID)
		if err != nil {
			return nil, err
		}
		if blocked {
			return &models.CardResult{
				Message:           fmt.Sprintf("%s's %s blocked the Jester's (%s) curse!", targetMention, protectionName(blockedByRedshirt), userMention),
				PointsDelta:       0,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: 0,
			}, nil
		}

		spareKeyBlocked, err := checkAndConsumeSpareKey(db, targetUser.ID, guildID)
		if err != nil {
			return nil, err
		}
		if spareKeyBlocked {
			return &models.CardResult{
				Message:           fmt.Sprintf("%s's Spare Key blocked the Jester's (%s) curse!", targetMention, userMention),
				PointsDelta:       0,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: 0,
			}, nil
		}

		timeoutUntil := time.Now().Add(15 * time.Minute)

		if err := s.GuildMemberTimeout(guildID, targetUserID, &timeoutUntil); err != nil {
			return &models.CardResult{
				Message:     fmt.Sprintf("%s failed to mute the target! They might be an Admin or too powerful.", userMention),
				PointsDelta: 0,
				PoolDelta:   0,
			}, nil
		}

		return &models.CardResult{
			Message:           fmt.Sprintf("The Jester laughs! %s has muted %s for 15 minutes.", userMention, targetMention),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: 0,
		}, nil
	}

	return &models.CardResult{
		Message:     "Target user not found in this server.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleGenerousDonation(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	var guild models.Guild
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return nil, err
	}

	moonCount := int64(0)
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, guildID, TheMoonCardID).
		Count(&moonCount).Error
	if err != nil {
		return nil, err
	}
	if moonCount > 0 {
		if _, err := CheckAndConsumeMoon(db, user.ID, guildID); err != nil {
			return nil, err
		}
		randomUserID, err := GetRandomUserForMoon(db, guildID, []uint{user.ID})
		if err != nil {
			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, user.ID, guildID)
			if err != nil {
				return nil, err
			}
			if blocked {
				if err := removeCardFromInventory(db, user.ID, guildID, GenerousDonationCardID); err != nil {
					return nil, fmt.Errorf("failed to remove blocked donation card: %v", err)
				}
				return &models.CardResult{
					Message:     fmt.Sprintf("Your Moon illusion tried to redirect, but no eligible users found. Your %s blocked the Generous Donation instead! The card fizzles out.", protectionName(blockedByRedshirt)),
					PointsDelta: 0,
					PoolDelta:   0,
				}, nil
			}
			return &models.CardResult{
				Message:     "Your Moon illusion tried to redirect, but no eligible users found. The card fizzles out.",
				PointsDelta: 0,
				PoolDelta:   0,
			}, nil
		}

		var randomUser models.User
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", randomUserID, guildID).
			First(&randomUser).Error; err != nil {
			return nil, err
		}

		standardCost := guild.CardDrawCost
		randomUser.Points += standardCost
		if err := db.Save(&randomUser).Error; err != nil {
			return nil, err
		}

		if err := removeCardFromInventory(db, user.ID, guildID, GenerousDonationCardID); err != nil {
			return nil, fmt.Errorf("failed to remove redirected donation card: %v", err)
		}

		randomMention := "<@" + randomUserID + ">"
		return &models.CardResult{
			Message:           fmt.Sprintf("Your Moon illusion redirected the Generous Donation! %s received %.0f points instead!", randomMention, standardCost),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &randomUserID,
			TargetPointsDelta: standardCost,
		}, nil
	}

	blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, user.ID, guildID)
	if err != nil {
		return nil, err
	}
	if blocked {
		if err := removeCardFromInventory(db, user.ID, guildID, GenerousDonationCardID); err != nil {
			return nil, fmt.Errorf("failed to remove blocked donation card: %v", err)
		}
		return &models.CardResult{
			Message:     fmt.Sprintf("Your %s blocked the Generous Donation! The card fizzles out.", protectionName(blockedByRedshirt)),
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	standardCost := guild.CardDrawCost

	var message string
	var pointsDelta float64
	var poolDelta float64

	if user.Points >= standardCost {
		pointsDelta = -standardCost
		poolDelta = standardCost
		message = fmt.Sprintf("You have generously paid %.0f points forward! The next user to draw a standard cost card will get it for free.", standardCost)
	} else {
		if err := removeCardFromInventory(db, user.ID, guildID, GenerousDonationCardID); err != nil {
			return nil, fmt.Errorf("failed to remove unfunded donation card: %v", err)
		}

		return &models.CardResult{
			Message:     fmt.Sprintf("You don't have enough points (%.0f) to make a generous donation! The card was returned.", standardCost),
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:     message,
		PointsDelta: pointsDelta,
		PoolDelta:   poolDelta,
	}, nil
}

func removeCardFromInventory(db *gorm.DB, userID uint, guildID string, cardID uint) error {
	var item models.UserInventory
	result := db.Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cardID).First(&item)
	if result.Error != nil {
		return result.Error
	}
	return db.Delete(&item).Error
}

func handleStimulusCheck(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var userCount int64
	if err := db.Model(&models.User{}).Where("guild_id = ?", guildID).Count(&userCount).Error; err != nil {
		return nil, err
	}

	if userCount == 0 {
		return &models.CardResult{
			Message:     "No users found in the server. The stimulus check bounces.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	gainAmount := 50.0
	var updatedCount int64
	if err := db.Model(&models.User{}).
		Where("guild_id = ? AND discord_id != ?", guildID, userID).
		Count(&updatedCount).Error; err != nil {
		return nil, err
	}
	if updatedCount > 0 {
		if err := db.Model(&models.User{}).
			Where("guild_id = ? AND discord_id != ?", guildID, userID).
			Update("points", gorm.Expr("points + ?", gainAmount)).Error; err != nil {
			return nil, err
		}
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("Stimulus Check arrived! Everyone in the server gained %.0f points! (%d users affected)", gainAmount, int(updatedCount)+1),
		PointsDelta: gainAmount,
		PoolDelta:   0,
	}, nil
}

func handleTheHierophant(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var userCount int64
	if err := db.Model(&models.User{}).Where("guild_id = ?", guildID).Count(&userCount).Error; err != nil {
		return nil, err
	}

	if userCount == 0 {
		return &models.CardResult{
			Message:     "No users found in the server. The blessing has no one to bless.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	guild, err := guildService.GetGuildInfo(s, db, guildID, "")
	if err != nil {
		return nil, err
	}

	gainAmount := 50.0
	var updatedCount int64
	if err := db.Model(&models.User{}).
		Where("guild_id = ? AND discord_id != ?", guildID, userID).
		Count(&updatedCount).Error; err != nil {
		return nil, err
	}
	if updatedCount > 0 {
		if err := db.Model(&models.User{}).
			Where("guild_id = ? AND discord_id != ?", guildID, userID).
			Update("points", gorm.Expr("points + ?", gainAmount)).Error; err != nil {
			return nil, err
		}
	}

	poolDrain := 500.0
	if guild.Pool < poolDrain {
		poolDrain = guild.Pool
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("The Hierophant's blessing! Everyone in the server gained %.0f points! (%d users affected) %.0f points were added to the pool with this blessing.", gainAmount, int(updatedCount)+1, poolDrain),
		PointsDelta: gainAmount,
		PoolDelta:   poolDrain,
	}, nil
}

func handleFactoryReset(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	if user.Points < 1000 {
		diff := 1000.0 - user.Points
		return &models.CardResult{
			Message:     fmt.Sprintf("Factory Reset! Your points were reset to 1000 (+%.0f points).", diff),
			PointsDelta: diff,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:     "Factory Reset triggered, but you have 1000 or more points. Factory Reset fizzles out.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleQuickFlip(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	var guild models.Guild
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return nil, err
	}

	var cardCost float64
	switch user.CardDrawCount {
	case 1:
		cardCost = guild.CardDrawCost
	case 2:
		cardCost = guild.CardDrawCost * 10
	default:
		cardCost = guild.CardDrawCost * 50
	}

	coinFlip := rand.Intn(2)

	if coinFlip == 1 {
		winnings := cardCost * 2
		return &models.CardResult{
			Message:     fmt.Sprintf("Heads! You doubled your card cost back and gained %.0f points!", winnings),
			PointsDelta: winnings,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:     "Tails! You get nothing.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleBetFreeze(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Bet Freeze requires you to select a target to freeze!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecuteBetFreeze(s *discordgo.Session, db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var targetUser models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
		First(&targetUser).Error; err != nil {
		return nil, err
	}
	targetMention := "<@" + targetUserID + ">"
	userMention := "<@" + userID + ">"

	moonRedirected, err := CheckAndConsumeMoon(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if moonRedirected {
		randomUserID, err := GetRandomUserForMoon(db, guildID, []uint{targetUser.ID, user.ID})
		if err != nil {
			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, targetUser.ID, guildID)
			if err != nil {
				return nil, err
			}
			if blocked {
				targetID := targetUserID
				return &models.CardResult{
					Message:           fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. %s blocked the Bet Freeze instead!", targetMention, protectionName(blockedByRedshirt)),
					PointsDelta:       0,
					PoolDelta:         0,
					TargetUserID:      &targetID,
					TargetPointsDelta: 0,
				}, nil
			}
			targetID := targetUserID
			return &models.CardResult{
				Message:           fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. The freeze fizzles out.", targetMention),
				PointsDelta:       0,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: 0,
			}, nil
		}

		var randomUser models.User
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", randomUserID, guildID).
			First(&randomUser).Error; err != nil {
			return nil, err
		}

		timeoutUntil := time.Now().Add(2 * time.Hour)
		randomUser.BetLockoutUntil = &timeoutUntil
		if err := db.Save(&randomUser).Error; err != nil {
			return nil, err
		}

		randomMention := "<@" + randomUserID + ">"
		targetID := randomUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's Moon illusion redirected the Bet Freeze! %s cannot place bets until %s instead!", targetMention, randomMention, timeoutUntil.Format("3:04 PM")),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: 0,
		}, nil
	}

	blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if blocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's %s blocked the Bet Freeze!", targetMention, protectionName(blockedByRedshirt)),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: 0,
		}, nil
	}

	spareKeyBlocked, err := checkAndConsumeSpareKey(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if spareKeyBlocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's Spare Key got them out of the Bet Freeze!", targetMention),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: 0,
		}, nil
	}

	lockoutUntil := time.Now().Add(2 * time.Hour)
	targetUser.BetLockoutUntil = &lockoutUntil

	if err := db.Save(&targetUser).Error; err != nil {
		return nil, err
	}

	targetID := targetUserID
	return &models.CardResult{
		Message:           fmt.Sprintf("%s has frozen %s's betting ability for 2 hours!", userMention, targetMention),
		PointsDelta:       0,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: 0,
	}, nil
}

func handleBetInsurance(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "Bet Insurance active! If you lose your next bet, you get 25% of your wager back.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleGreenShells(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var allUsers []models.User
		if err := tx.Where("guild_id = ? AND discord_id != ?", guildID, userID).Find(&allUsers).Error; err != nil {
			return err
		}

		if len(allUsers) == 0 {
			result = &models.CardResult{
				Message:     "No other users found to target with Green Shells. The shells break against the wall.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		rand.Shuffle(len(allUsers), func(i, j int) {
			allUsers[i], allUsers[j] = allUsers[j], allUsers[i]
		})

		numTargets := 3
		if len(allUsers) < 3 {
			numTargets = len(allUsers)
		}

		targets := allUsers[:numTargets]
		var message string

		for _, target := range targets {
			var lockedTarget models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedTarget, target.ID).Error; err != nil {
				return err
			}

			moonRedirected, err := CheckAndConsumeMoon(tx, lockedTarget.ID, guildID)
			if err != nil {
				return err
			}
			if moonRedirected {
				var allUsers []models.User
				if err := tx.Where("guild_id = ? AND deleted_at IS NULL AND id != ?", guildID, lockedTarget.ID).Find(&allUsers).Error; err != nil {
					return err
				}
				if len(allUsers) == 0 {
					blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedTarget.ID, guildID)
					if err != nil {
						return err
					}
					targetName := lockedTarget.Username
					displayName := ""
					if targetName == nil || *targetName == "" {
						displayName = fmt.Sprintf("<@%s>", lockedTarget.DiscordID)
					} else {
						displayName = *targetName
					}
					if blocked {
						message += fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. %s blocked a shell! ", displayName, protectionName(blockedByRedshirt))
					} else {
						message += fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. ", displayName)
					}
					continue
				}

				randomIndex := rand.Intn(len(allUsers))
				randomTarget := allUsers[randomIndex]
				var randomLockedTarget models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&randomLockedTarget, randomTarget.ID).Error; err != nil {
					return err
				}

				loss := float64(rand.Intn(25) + 1)
				if randomLockedTarget.Points < loss {
					loss = randomLockedTarget.Points
				}

				randomLockedTarget.Points -= loss
				if err := tx.Save(&randomLockedTarget).Error; err != nil {
					return err
				}

				targetName := lockedTarget.Username
				displayName := ""
				if targetName == nil || *targetName == "" {
					displayName = fmt.Sprintf("<@%s>", lockedTarget.DiscordID)
				} else {
					displayName = *targetName
				}

				randomName := randomLockedTarget.Username
				randomDisplayName := ""
				if randomName == nil || *randomName == "" {
					randomDisplayName = fmt.Sprintf("<@%s>", randomLockedTarget.DiscordID)
				} else {
					randomDisplayName = *randomName
				}

				message += fmt.Sprintf("%s's Moon illusion redirected a shell to %s! %s lost %.0f points! ", displayName, randomDisplayName, randomDisplayName, loss)
				continue
			}

			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedTarget.ID, guildID)
			if err != nil {
				return err
			}

			displayName := fmt.Sprintf("<@%s>", lockedTarget.DiscordID)

			if blocked {
				message += fmt.Sprintf("%s's %s blocked a shell! ", displayName, protectionName(blockedByRedshirt))
				continue
			}

			loss := float64(rand.Intn(25) + 1)

			if lockedTarget.Points < loss {
				loss = lockedTarget.Points
			}

			lockedTarget.Points -= loss
			if err := tx.Save(&lockedTarget).Error; err != nil {
				return err
			}

			message += fmt.Sprintf("%s was hit for %.0f points! ", displayName, loss)
		}

		if message == "" {
			message = "Green Shells were thrown but missed everyone (or were all blocked)!"
		} else {
			message = "Green Shells thrown! " + message
		}

		result = &models.CardResult{
			Message:     message,
			PointsDelta: 0,
			PoolDelta:   0,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleWhackAMole(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var allUsers []models.User
		if err := tx.Where("guild_id = ? AND discord_id != ?", guildID, userID).Find(&allUsers).Error; err != nil {
			return err
		}

		if len(allUsers) == 0 {
			result = &models.CardResult{
				Message:     "No moles found to whack! The hammer hits the empty ground.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		rand.Shuffle(len(allUsers), func(i, j int) {
			allUsers[i], allUsers[j] = allUsers[j], allUsers[i]
		})

		maxTargets := 5
		minTargets := 3

		numTargets := rand.Intn(maxTargets-minTargets+1) + minTargets

		if len(allUsers) < numTargets {
			numTargets = len(allUsers)
		}

		targets := allUsers[:numTargets]
		var message string

		for _, target := range targets {
			var lockedTarget models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedTarget, target.ID).Error; err != nil {
				return err
			}

			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedTarget.ID, guildID)
			if err != nil {
				return err
			}

			targetName := lockedTarget.Username
			displayName := ""
			if targetName == nil || *targetName == "" {
				displayName = fmt.Sprintf("<@%s>", lockedTarget.DiscordID)
			} else {
				displayName = *targetName
			}

			if blocked {
				message += fmt.Sprintf("%s's %s blocked the hammer! ", displayName, protectionName(blockedByRedshirt))
				continue
			}

			loss := float64(rand.Intn(10) + 1)

			if lockedTarget.Points < loss {
				loss = lockedTarget.Points
			}

			lockedTarget.Points -= loss
			if err := tx.Save(&lockedTarget).Error; err != nil {
				return err
			}

			message += fmt.Sprintf("%s whacked for %.0f! ", displayName, loss)
		}

		if message == "" {
			message = "You swung the hammer but hit nothing (or all blocked)!"
		} else {
			message = "Whack-a-Mole! " + message
		}

		result = &models.CardResult{
			Message:     message,
			PointsDelta: 0,
			PoolDelta:   0,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleUnoReverse(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var count int64
	err := db.Table("bet_entries").
		Joins("JOIN bets ON bets.id = bet_entries.bet_id").
		Where("bet_entries.user_id = ? AND bets.paid = ? and bet_entries.deleted_at is null", user.ID, false).
		Count(&count).Error

	if err != nil {
		return nil, err
	}

	if count == 0 {
		return &models.CardResult{
			Message:     "You have no active bets to use Uno Reverse on! The card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:           "Select an active bet to use Uno Reverse on!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "bet",
	}, nil
}

func handleLoanShark(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You took a loan! +500 points. You will automatically lose 600 points in 3 days.",
		PointsDelta: 500,
		PoolDelta:   0,
	}, nil
}

func handleSocialism(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var allUsers []models.User
		if err := tx.Where("guild_id = ? and deleted_at is null", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
			return err
		}

		if len(allUsers) == 0 {
			result = &models.CardResult{
				Message:     "No players found in the server. Socialism has no one to redistribute from.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		numTopPlayers := 3
		if len(allUsers) < numTopPlayers {
			numTopPlayers = len(allUsers)
		}
		topPlayers := allUsers[:numTopPlayers]

		var bottomPlayers []models.User
		topPlayerIDs := make(map[uint]bool)
		for _, topPlayer := range topPlayers {
			topPlayerIDs[topPlayer.ID] = true
		}

		for i := len(allUsers) - 1; i >= 0 && len(bottomPlayers) < 3; i-- {
			if !topPlayerIDs[allUsers[i].ID] {
				bottomPlayers = append(bottomPlayers, allUsers[i])
			}
		}

		for i, j := 0, len(bottomPlayers)-1; i < j; i, j = i+1, j-1 {
			bottomPlayers[i], bottomPlayers[j] = bottomPlayers[j], bottomPlayers[i]
		}

		if len(topPlayers) == 0 {
			result = &models.CardResult{
				Message:     "No top players found to take points from. Socialism fizzles out.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		if len(bottomPlayers) == 0 {
			result = &models.CardResult{
				Message:     "No bottom players found to give points to. Socialism fizzles out.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		var totalCollected float64
		var topMessage string
		var bottomMessage string

		for _, topPlayer := range topPlayers {
			var lockedTop models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedTop, topPlayer.ID).Error; err != nil {
				return err
			}

			topPlayerName := lockedTop.Username
			topDisplayName := ""
			if topPlayerName == nil || *topPlayerName == "" {
				topDisplayName = fmt.Sprintf("<@%s>", lockedTop.DiscordID)
			} else {
				topDisplayName = *topPlayerName
			}

			takeAmount := 90.0
			if lockedTop.Points < takeAmount {
				takeAmount = lockedTop.Points
			}

			moonRedirected, err := CheckAndConsumeMoon(tx, lockedTop.ID, guildID)
			if err != nil {
				return err
			}
			if moonRedirected {
				randomDiscordID, err := GetRandomUserForMoon(tx, guildID, []uint{lockedTop.ID})
				if err != nil {
					blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedTop.ID, guildID)
					if err != nil {
						return err
					}
					if blocked {
						topMessage += fmt.Sprintf("%s's Moon had no one to redirect to; %s blocked!\n", topDisplayName, protectionName(blockedByRedshirt))
					} else {
						topMessage += fmt.Sprintf("%s's Moon had no one to redirect to; fizzles.\n", topDisplayName)
					}
					continue
				}
				var randomUser models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("discord_id = ? AND guild_id = ?", randomDiscordID, guildID).First(&randomUser).Error; err != nil {
					return err
				}
				actualTake := takeAmount
				if randomUser.Points < actualTake {
					actualTake = randomUser.Points
				}
				if actualTake > 0 {
					randomUser.Points -= actualTake
					if randomUser.Points < 0 {
						randomUser.Points = 0
					}
					if err := tx.Save(&randomUser).Error; err != nil {
						return err
					}
					totalCollected += actualTake
					randomDisplayName := fmt.Sprintf("<@%s>", randomUser.DiscordID)
					if randomUser.Username != nil && *randomUser.Username != "" {
						randomDisplayName = *randomUser.Username
					}
					topMessage += fmt.Sprintf("%s's Moon redirected! %s lost %.0f points\n", topDisplayName, randomDisplayName, actualTake)
				}
				continue
			}

			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedTop.ID, guildID)
			if err != nil {
				return err
			}

			if blocked {
				topMessage += fmt.Sprintf("%s's %s blocked!\n", topDisplayName, protectionName(blockedByRedshirt))
				continue
			}

			if takeAmount > 0 {
				lockedTop.Points -= takeAmount
				totalCollected += takeAmount
				if err := tx.Save(&lockedTop).Error; err != nil {
					return err
				}
				topMessage += fmt.Sprintf("%s lost %.0f points\n", topDisplayName, takeAmount)
			}
		}

		if totalCollected > 0 && len(bottomPlayers) > 0 {
			amountPerBottomPlayer := totalCollected / float64(len(bottomPlayers))

			for _, bottomPlayer := range bottomPlayers {
				var lockedBottom models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedBottom, bottomPlayer.ID).Error; err != nil {
					return err
				}

				lockedBottom.Points += amountPerBottomPlayer
				if err := tx.Save(&lockedBottom).Error; err != nil {
					return err
				}

				bottomPlayerName := lockedBottom.Username
				bottomDisplayName := ""
				if bottomPlayerName == nil || *bottomPlayerName == "" {
					bottomDisplayName = fmt.Sprintf("<@%s>", lockedBottom.DiscordID)
				} else {
					bottomDisplayName = *bottomPlayerName
				}

				bottomMessage += fmt.Sprintf("%s gained %.0f points\n", bottomDisplayName, amountPerBottomPlayer)
			}
		}

		var message string
		if topMessage != "" {
			message = "Socialism activated! Top players:\n" + topMessage
		}
		if bottomMessage != "" {
			message += "Bottom players: \n" + bottomMessage
		}

		message = strings.TrimSuffix(message, " ")

		if message == "" {
			message = "Socialism activated but no redistribution occurred (all top players were shielded or had no points)."
		}

		result = &models.CardResult{
			Message:     message,
			PointsDelta: 0,
			PoolDelta:   0,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleRobinHood(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var allUsers []models.User
		if err := tx.Where("guild_id = ? and deleted_at is null", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
			return err
		}

		if len(allUsers) == 0 {
			result = &models.CardResult{
				Message:     "No players found in the server. Robin Hood has no one to rob from.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		if len(allUsers) == 1 {
			result = &models.CardResult{
				Message:     "Only one player in the server. Robin Hood fizzles out.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		topPlayer := allUsers[0]
		bottomPlayer := allUsers[len(allUsers)-1]

		if topPlayer.ID == bottomPlayer.ID {
			result = &models.CardResult{
				Message:     "The richest and poorest players are the same! Robin Hood has no one to redistribute from.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		var lockedTop models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedTop, topPlayer.ID).Error; err != nil {
			return err
		}
		var lockedBottom models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedBottom, bottomPlayer.ID).Error; err != nil {
			return err
		}

		topPlayerName := lockedTop.Username
		topDisplayName := ""
		if topPlayerName == nil || *topPlayerName == "" {
			topDisplayName = fmt.Sprintf("<@%s>", lockedTop.DiscordID)
		} else {
			topDisplayName = *topPlayerName
		}

		bottomPlayerName := lockedBottom.Username
		bottomDisplayName := ""
		if bottomPlayerName == nil || *bottomPlayerName == "" {
			bottomDisplayName = fmt.Sprintf("<@%s>", lockedBottom.DiscordID)
		} else {
			bottomDisplayName = *bottomPlayerName
		}

		moonRedirected, err := CheckAndConsumeMoon(tx, lockedTop.ID, guildID)
		if err != nil {
			return err
		}
		if moonRedirected {
			var allUsers []models.User
			if err := tx.Where("guild_id = ? AND deleted_at IS NULL AND id != ?", guildID, lockedTop.ID).Find(&allUsers).Error; err != nil {
				return err
			}
			if len(allUsers) == 0 {
				blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedTop.ID, guildID)
				if err != nil {
					return err
				}
				if blocked {
					result = &models.CardResult{
						Message:     fmt.Sprintf("Robin Hood attempted to steal from %s, but their Moon illusion tried to redirect with no eligible users. %s parried the thief instead! The card fizzles out.", topDisplayName, protectionName(blockedByRedshirt)),
						PointsDelta: 0,
						PoolDelta:   0,
					}
				} else {
					result = &models.CardResult{
						Message:     fmt.Sprintf("Robin Hood attempted to steal from %s, but their Moon illusion tried to redirect with no eligible users. The card fizzles out.", topDisplayName),
						PointsDelta: 0,
						PoolDelta:   0,
					}
				}
				return nil
			}

			randomIndex := rand.Intn(len(allUsers))
			randomTarget := allUsers[randomIndex]
			var randomUser models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&randomUser, randomTarget.ID).Error; err != nil {
				return err
			}

			takeAmount := 200.0
			if randomUser.Points < takeAmount {
				takeAmount = randomUser.Points
			}

			if takeAmount <= 0 {
				result = &models.CardResult{
					Message:     fmt.Sprintf("Robin Hood attempted to steal from %s, but their Moon illusion redirected to someone with no points! The card fizzles out.", topDisplayName),
					PointsDelta: 0,
					PoolDelta:   0,
				}
				return nil
			}

			randomUser.Points -= takeAmount
			if err := tx.Save(&randomUser).Error; err != nil {
				return err
			}

			randomUsername := randomUser.Username
			randomDisplayName := ""
			if randomUsername == nil || *randomUsername == "" {
				randomDisplayName = fmt.Sprintf("<@%s>", randomUser.DiscordID)
			} else {
				randomDisplayName = *randomUsername
			}

			result = &models.CardResult{
				Message:           fmt.Sprintf("Robin Hood attempted to steal from %s, but their Moon illusion redirected it! %s lost %.0f points instead!", topDisplayName, randomDisplayName, takeAmount),
				PointsDelta:       0,
				PoolDelta:         0,
				TargetUserID:      &randomUser.DiscordID,
				TargetPointsDelta: -takeAmount,
			}
			return nil
		}

		blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedTop.ID, guildID)
		if err != nil {
			return err
		}

		if blocked {
			result = &models.CardResult{
				Message:     fmt.Sprintf("Robin Hood attempted to steal from %s, but their %s parried the thief! The card fizzles out.", topDisplayName, protectionName(blockedByRedshirt)),
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		takeAmount := 200.0
		if lockedTop.Points < takeAmount {
			takeAmount = lockedTop.Points
		}

		if takeAmount <= 0 {
			result = &models.CardResult{
				Message:     fmt.Sprintf("Robin Hood attempted to steal from %s, but they have no points! The card fizzles out.", topDisplayName),
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		lockedTop.Points -= takeAmount
		if err := tx.Save(&lockedTop).Error; err != nil {
			return err
		}

		lockedBottom.Points += 150.0
		if err := tx.Save(&lockedBottom).Error; err != nil {
			return err
		}

		message := fmt.Sprintf("Robin Hood strikes! Stole %.0f points from %s, gave 150 to %s, and kept 50 for yourself!", takeAmount, topDisplayName, bottomDisplayName)

		result = &models.CardResult{
			Message:     message,
			PointsDelta: 50.0,
			PoolDelta:   0,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleRedShells(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var allUsers []models.User
		if err := tx.Where("guild_id = ? and deleted_at is null", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
			return err
		}

		if len(allUsers) == 0 {
			result = &models.CardResult{
				Message:     "No players found in the server. Red Shells have no targets.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		var drawerIndex int = -1
		for i, user := range allUsers {
			if user.DiscordID == userID {
				drawerIndex = i
				break
			}
		}

		if drawerIndex == -1 {
			result = &models.CardResult{
				Message:     "Could not find your position on the leaderboard. Red Shells break against the wall.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		if drawerIndex == 0 {
			result = &models.CardResult{
				Message:     "You're at the top of the leaderboard! There's no one in front of you to hit with Red Shells. Red Shells break against the wall.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		numTargets := 3
		if drawerIndex < numTargets {
			numTargets = drawerIndex
		}

		var targets []models.User
		for i := drawerIndex - numTargets; i < drawerIndex; i++ {
			targets = append(targets, allUsers[i])
		}

		if len(targets) == 0 {
			result = &models.CardResult{
				Message:     "No targets found in front of you. Red Shells break against the wall.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		var message string

		for _, target := range targets {
			var lockedTarget models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedTarget, target.ID).Error; err != nil {
				return err
			}

			moonRedirected, err := CheckAndConsumeMoon(tx, lockedTarget.ID, guildID)
			if err != nil {
				return err
			}
			if moonRedirected {
				var allUsers []models.User
				if err := tx.Where("guild_id = ? AND deleted_at IS NULL AND id != ?", guildID, lockedTarget.ID).Find(&allUsers).Error; err != nil {
					return err
				}
				if len(allUsers) == 0 {
					blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedTarget.ID, guildID)
					if err != nil {
						return err
					}
					targetName := lockedTarget.Username
					displayName := ""
					if targetName == nil || *targetName == "" {
						displayName = fmt.Sprintf("<@%s>", lockedTarget.DiscordID)
					} else {
						displayName = *targetName
					}
					if blocked {
						message += fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. %s blocked a shell! ", displayName, protectionName(blockedByRedshirt))
					} else {
						message += fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. ", displayName)
					}
					continue
				}

				randomIndex := rand.Intn(len(allUsers))
				randomTarget := allUsers[randomIndex]
				var randomLockedTarget models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&randomLockedTarget, randomTarget.ID).Error; err != nil {
					return err
				}

				loss := float64(rand.Intn(26) + 25)
				if randomLockedTarget.Points < loss {
					loss = randomLockedTarget.Points
				}

				randomLockedTarget.Points -= loss
				if err := tx.Save(&randomLockedTarget).Error; err != nil {
					return err
				}

				targetName := lockedTarget.Username
				displayName := ""
				if targetName == nil || *targetName == "" {
					displayName = fmt.Sprintf("<@%s>", lockedTarget.DiscordID)
				} else {
					displayName = *targetName
				}

				randomName := randomLockedTarget.Username
				randomDisplayName := ""
				if randomName == nil || *randomName == "" {
					randomDisplayName = fmt.Sprintf("<@%s>", randomLockedTarget.DiscordID)
				} else {
					randomDisplayName = *randomName
				}

				message += fmt.Sprintf("%s's Moon illusion redirected a shell to %s! %s lost %.0f points! ", displayName, randomDisplayName, randomDisplayName, loss)
				continue
			}

			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedTarget.ID, guildID)
			if err != nil {
				return err
			}

			targetName := lockedTarget.Username
			displayName := ""
			if targetName == nil || *targetName == "" {
				displayName = fmt.Sprintf("<@%s>", lockedTarget.DiscordID)
			} else {
				displayName = *targetName
			}

			if blocked {
				message += fmt.Sprintf("%s's %s blocked a shell! ", displayName, protectionName(blockedByRedshirt))
				continue
			}

			loss := float64(rand.Intn(26) + 25)

			if lockedTarget.Points < loss {
				loss = lockedTarget.Points
			}

			if loss > 0 {
				lockedTarget.Points -= loss
				if err := tx.Save(&lockedTarget).Error; err != nil {
					return err
				}
				message += fmt.Sprintf("%s was hit for %.0f points! ", displayName, loss)
			}
		}

		if message == "" {
			message = "Red Shells were thrown but all were blocked!"
		} else {
			message = "Red Shells thrown! " + message
		}

		result = &models.CardResult{
			Message:     message,
			PointsDelta: 0,
			PoolDelta:   0,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func protectionName(blockedByRedshirt bool) string {
	if blockedByRedshirt {
		return "Redshirt"
	}
	return "Shield"
}

// CheckAndConsumeShield removes one Shield from the user's inventory if present; returns true if consumed.
func CheckAndConsumeShield(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ? and deleted_at is null", userID, guildID, ShieldCardID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		if err := removeCardFromInventory(db, userID, guildID, ShieldCardID); err != nil {
			return true, fmt.Errorf("failed to consume shield: %v", err)
		}

		return true, nil
	}
	return false, nil
}

// burnExpiredRedshirt soft-deletes one expired Redshirt for the user, if any.
// Expired = (expires_at IS NOT NULL AND expires_at < now) OR (expires_at IS NULL AND created_at < now - 2h).
func burnExpiredRedshirt(db *gorm.DB, userID uint, guildID string) error {
	now := time.Now()
	legacyCutoff := now.Add(-2 * time.Hour)
	var item models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL", userID, guildID, RedshirtCardID).
		Where("(expires_at IS NOT NULL AND expires_at < ?) OR (expires_at IS NULL AND created_at < ?)", now, legacyCutoff).
		First(&item).Error
	if err != nil {
		return nil
	}
	return db.Delete(&item).Error
}

// HasActiveRedshirt returns true if the user has at least one Redshirt that is still active.
// Active = (expires_at IS NOT NULL AND expires_at > now) OR (expires_at IS NULL AND created_at >= now - 2h).
func HasActiveRedshirt(db *gorm.DB, userID uint, guildID string) (bool, error) {
	now := time.Now()
	legacyCutoff := now.Add(-2 * time.Hour)
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL", userID, guildID, RedshirtCardID).
		Where("(expires_at IS NOT NULL AND expires_at > ?) OR (expires_at IS NULL AND created_at >= ?)", now, legacyCutoff).
		Count(&count).Error
	return count > 0, err
}

// BurnExpiredRedshirts soft-deletes all expired Redshirt rows for the user.
func BurnExpiredRedshirts(db *gorm.DB, userID uint, guildID string) error {
	now := time.Now()
	legacyCutoff := now.Add(-2 * time.Hour)
	var items []models.UserInventory
	if err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL", userID, guildID, RedshirtCardID).
		Where("(expires_at IS NOT NULL AND expires_at < ?) OR (expires_at IS NULL AND created_at < ?)", now, legacyCutoff).
		Find(&items).Error; err != nil {
		return err
	}
	for i := range items {
		if err := db.Delete(&items[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// CheckAndConsumeShieldOrRedshirt checks Redshirt first (active = block without consume); if expired, burns it and falls through to Shield.
// Returns (blocked, blockedByRedshirt, err). When blockedByRedshirt is true, Shield was not consumed.
func CheckAndConsumeShieldOrRedshirt(db *gorm.DB, userID uint, guildID string) (blocked bool, blockedByRedshirt bool, err error) {
	var redshirt models.UserInventory
	err = db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL", userID, guildID, RedshirtCardID).
		Order("created_at DESC").Limit(1).First(&redshirt).Error
	if err != nil {
		// If error is not ErrRecordNotFound, return immediately
		if err != gorm.ErrRecordNotFound {
			return false, false, err
		}
		// If NotFound, fall through to CheckAndConsumeShield
	} else {
		// Record found - check if active
		active := false
		if redshirt.ExpiresAt != nil {
			active = time.Now().Before(*redshirt.ExpiresAt)
		} else {
			active = time.Since(redshirt.CreatedAt) < 2*time.Hour
		}
		if active {
			return true, true, nil
		}
		// Expired - delete it and propagate any error
		if err = db.Delete(&redshirt).Error; err != nil {
			return false, false, err
		}
		// Deletion succeeded, fall through to CheckAndConsumeShield
	}
	blocked, err = CheckAndConsumeShield(db, userID, guildID)
	return blocked, false, err
}

func checkAndConsumeSpareKey(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ? and deleted_at is null", userID, guildID, SpareKeyCardID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		if err := removeCardFromInventory(db, userID, guildID, SpareKeyCardID); err != nil {
			return true, fmt.Errorf("failed to consume spare key: %v", err)
		}

		return true, nil
	}
	return false, nil
}

func CheckAndConsumeMoon(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ? and deleted_at is null", userID, guildID, TheMoonCardID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		if err := removeCardFromInventory(db, userID, guildID, TheMoonCardID); err != nil {
			return true, fmt.Errorf("failed to consume moon: %v", err)
		}

		return true, nil
	}
	return false, nil
}

func GetRandomUserForMoon(db *gorm.DB, guildID string, excludeUserIDs []uint) (string, error) {
	var allUsers []models.User
	query := db.Where("guild_id = ? AND deleted_at IS NULL", guildID)

	if len(excludeUserIDs) > 0 {
		query = query.Where("id NOT IN ?", excludeUserIDs)
	}

	if err := query.Find(&allUsers).Error; err != nil {
		return "", err
	}

	if len(allUsers) == 0 {
		return "", fmt.Errorf("no eligible users found")
	}

	randomIndex := rand.Intn(len(allUsers))
	return allUsers[randomIndex].DiscordID, nil
}

func ExecuteHostileTakeover(tx *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	drawerMention := "<@" + userID + ">"
	err := func() error {
		var drawer models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&drawer).Error; err != nil {
			return err
		}
		var target models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
			First(&target).Error; err != nil {
			return err
		}
		drawerOrig := drawer.Points
		targetOrig := target.Points
		targetMention := "<@" + target.DiscordID + ">"
		targetDisplay := targetMention
		if target.Username != nil && *target.Username != "" {
			targetDisplay = *target.Username
		}

		if targetOrig > drawerOrig {
			moonRedirected, err := CheckAndConsumeMoon(tx, target.ID, guildID)
			if err != nil {
				return err
			}
			if moonRedirected {
				randomDiscordID, err := GetRandomUserForMoon(tx, guildID, []uint{drawer.ID, target.ID})
				if err != nil {
					blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, target.ID, guildID)
					if err != nil {
						return err
					}
					if blocked {
						result = &models.CardResult{
							Message:           fmt.Sprintf("Hostile Takeover: %s's Moon had no one to redirect to and their %s blocked the swap! The card fizzles.", targetMention, protectionName(blockedByRedshirt)),
							PointsDelta:       0,
							PoolDelta:         0,
							TargetUserID:      &targetUserID,
							TargetPointsDelta: 0,
						}
					} else {
						result = &models.CardResult{
							Message:           fmt.Sprintf("Hostile Takeover: %s's Moon had no one to redirect to. The card fizzles.", targetMention),
							PointsDelta:       0,
							PoolDelta:         0,
							TargetUserID:      &targetUserID,
							TargetPointsDelta: 0,
						}
					}
					return nil
				}
				var randomUser models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("discord_id = ? AND guild_id = ?", randomDiscordID, guildID).First(&randomUser).Error; err != nil {
					return err
				}
				randomOrig := randomUser.Points
				drawer.Points, randomUser.Points = randomUser.Points, drawer.Points
				if err := tx.Save(&drawer).Error; err != nil {
					return err
				}
				if err := tx.Save(&randomUser).Error; err != nil {
					return err
				}
				randomMention := "<@" + randomUser.DiscordID + ">"
				result = &models.CardResult{
					Message:           fmt.Sprintf("Hostile Takeover: %s's Moon redirected the swap! %s swapped points with %s instead.", targetMention, drawerMention, randomMention),
					PointsDelta:       randomOrig - drawerOrig,
					PoolDelta:         0,
					TargetUserID:      &randomUser.DiscordID,
					TargetPointsDelta: drawerOrig - randomOrig,
				}
				return nil
			}
			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, target.ID, guildID)
			if err != nil {
				return err
			}
			if blocked {
				result = &models.CardResult{
					Message:           fmt.Sprintf("Hostile Takeover: %s's %s blocked the swap! The card fizzles.", targetMention, protectionName(blockedByRedshirt)),
					PointsDelta:       0,
					PoolDelta:         0,
					TargetUserID:      &targetUserID,
					TargetPointsDelta: 0,
				}
				return nil
			}
		} else if drawerOrig > targetOrig {
			moonRedirected, err := CheckAndConsumeMoon(tx, drawer.ID, guildID)
			if err != nil {
				return err
			}
			if moonRedirected {
				randomDiscordID, err := GetRandomUserForMoon(tx, guildID, []uint{drawer.ID, target.ID})
				if err != nil {
					blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, drawer.ID, guildID)
					if err != nil {
						return err
					}
					if blocked {
						result = &models.CardResult{
							Message:           fmt.Sprintf("Hostile Takeover: %s's Moon had no one to redirect to and their %s blocked the swap! The card fizzles.", drawerMention, protectionName(blockedByRedshirt)),
							PointsDelta:       0,
							PoolDelta:         0,
							TargetUserID:      &targetUserID,
							TargetPointsDelta: 0,
						}
					} else {
						result = &models.CardResult{
							Message:           fmt.Sprintf("Hostile Takeover: %s's Moon had no one to redirect to. The card fizzles.", drawerMention),
							PointsDelta:       0,
							PoolDelta:         0,
							TargetUserID:      &targetUserID,
							TargetPointsDelta: 0,
						}
					}
					return nil
				}
				var randomUser models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("discord_id = ? AND guild_id = ?", randomDiscordID, guildID).First(&randomUser).Error; err != nil {
					return err
				}
				randomOrig := randomUser.Points
				drawer.Points, randomUser.Points = randomUser.Points, drawer.Points
				if err := tx.Save(&drawer).Error; err != nil {
					return err
				}
				if err := tx.Save(&randomUser).Error; err != nil {
					return err
				}
				randomMention := "<@" + randomUser.DiscordID + ">"
				result = &models.CardResult{
					Message:           fmt.Sprintf("Hostile Takeover: %s's Moon redirected the swap! %s swapped points with %s instead of %s.", drawerMention, drawerMention, randomMention, targetDisplay),
					PointsDelta:       randomOrig - drawerOrig,
					PoolDelta:         0,
					TargetUserID:      &randomUser.DiscordID,
					TargetPointsDelta: drawerOrig - randomOrig,
				}
				return nil
			}
			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, drawer.ID, guildID)
			if err != nil {
				return err
			}
			if blocked {
				result = &models.CardResult{
					Message:           fmt.Sprintf("Hostile Takeover: %s's %s blocked the swap! The card fizzles.", drawerMention, protectionName(blockedByRedshirt)),
					PointsDelta:       0,
					PoolDelta:         0,
					TargetUserID:      &targetUserID,
					TargetPointsDelta: 0,
				}
				return nil
			}
		}

		drawer.Points, target.Points = target.Points, drawer.Points
		if err := tx.Save(&drawer).Error; err != nil {
			return err
		}
		if err := tx.Save(&target).Error; err != nil {
			return err
		}
		result = &models.CardResult{
			Message:           fmt.Sprintf("Hostile Takeover successful! %s swapped points with %s.", drawerMention, targetDisplay),
			PointsDelta:       targetOrig - drawerOrig,
			PoolDelta:         0,
			TargetUserID:      &targetUserID,
			TargetPointsDelta: drawerOrig - targetOrig,
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func handleGrandLarceny(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Grand Larceny requires you to select a target!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func handleTheGossip(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "The Gossip requires you to select a target!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecuteTheGossip(s *discordgo.Session, db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var targetUser models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).First(&targetUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &models.CardResult{
				Message:     "Target user not found in this server.",
				PointsDelta: 0,
				PoolDelta:   0,
			}, nil
		}
		return nil, err
	}

	targetID := targetUserID
	targetMention := "<@" + targetUserID + ">"
	userMention := "<@" + userID + ">"
	return &models.CardResult{
		Message:           fmt.Sprintf("The Gossip spreads! %s has revealed that %s's current point balance is **%.1f points**.", userMention, targetMention, targetUser.Points),
		PointsDelta:       0,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: 0,
	}, nil
}

func handleAntiAntiBet(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Anti-Anti-Bet requires you to select a target user!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func handleVampire(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You've drawn The Vampire! For the next 24 hours, you'll earn 5% of every bet won by other players.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleTheEmperor(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     fmt.Sprintf("<@%s> drew The Emperor! Use /play-card to play this card and gain Authority for 1 hour: 10%% of all points won by other players will be diverted to the pool.", userID),
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleLeech(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You've drawn The Leech! For the next 12 hours, you'll siphon 1% of the richest player's points every hour.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleAlgaeBloom(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var guild models.Guild
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("guild_id = ?", guildID).
		First(&guild).Error; err != nil {
		return nil, err
	}

	now := time.Now()
	twentyFourHoursLater := now.Add(24 * time.Hour)

	var message string
	if guild.PoolDrainUntil != nil && now.Before(*guild.PoolDrainUntil) {
		message = "Algae Bloom was extended! The pool drain effect will continue for another 24 hours from now."
	} else {
		message = "Algae has taken over the pool! For the next 24 hours, every card drawn removes 100 points from the pool."
	}

	guild.PoolDrainUntil = &twentyFourHoursLater

	if err := db.Save(&guild).Error; err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     message,
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleHostileTakeover(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Hostile Takeover requires you to select a target user within 500 points of you!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func handleBlueShell(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var firstPlaceUser models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			Order("points DESC").
			First(&firstPlaceUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				result = &models.CardResult{
					Message:     "No players found in the server. The Blue Shell breaks against the wall.",
					PointsDelta: 0,
					PoolDelta:   0,
				}
				return nil
			}
			return err
		}

		if firstPlaceUser.DiscordID == userID {
			result = &models.CardResult{
				Message:     "You're in 1st place! The Blue Shell targets you, but you're already at the top. Blue Shell breaks against the wall.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		moonRedirected, err := CheckAndConsumeMoon(tx, firstPlaceUser.ID, guildID)
		if err != nil {
			return err
		}
		if moonRedirected {
			var allUsers []models.User
			if err := tx.Where("guild_id = ? AND deleted_at IS NULL AND id != ?", guildID, firstPlaceUser.ID).Find(&allUsers).Error; err != nil {
				return err
			}
			if len(allUsers) == 0 {
				blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, firstPlaceUser.ID, guildID)
				if err != nil {
					return err
				}
				firstPlaceUsername := firstPlaceUser.Username
				displayName := ""
				if firstPlaceUsername == nil || *firstPlaceUsername == "" {
					displayName = fmt.Sprintf("<@%s>", firstPlaceUser.DiscordID)
				} else {
					displayName = *firstPlaceUsername
				}
				if blocked {
					result = &models.CardResult{
						Message:     fmt.Sprintf("The Blue Shell was thrown at %s, but their Moon illusion tried to redirect with no eligible users. %s blocked it instead!", displayName, protectionName(blockedByRedshirt)),
						PointsDelta: 0,
						PoolDelta:   0,
					}
				} else {
					result = &models.CardResult{
						Message:     fmt.Sprintf("The Blue Shell was thrown at %s, but their Moon illusion tried to redirect with no eligible users. The shell fizzles out.", displayName),
						PointsDelta: 0,
						PoolDelta:   0,
					}
				}
				return nil
			}

			randomIndex := rand.Intn(len(allUsers))
			randomTarget := allUsers[randomIndex]
			var randomUser models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&randomUser, randomTarget.ID).Error; err != nil {
				return err
			}

			percentage := 0.1
			deductAmount := randomUser.Points * percentage
			if randomUser.Points < deductAmount {
				deductAmount = randomUser.Points
			}

			randomUser.Points -= deductAmount
			if err := tx.Save(&randomUser).Error; err != nil {
				return err
			}

			firstPlaceUsername := firstPlaceUser.Username
			displayName := ""
			if firstPlaceUsername == nil || *firstPlaceUsername == "" {
				displayName = fmt.Sprintf("<@%s>", firstPlaceUser.DiscordID)
			} else {
				displayName = *firstPlaceUsername
			}

			randomUsername := randomUser.Username
			randomDisplayName := ""
			if randomUsername == nil || *randomUsername == "" {
				randomDisplayName = fmt.Sprintf("<@%s>", randomUser.DiscordID)
			} else {
				randomDisplayName = *randomUsername
			}

			result = &models.CardResult{
				Message:           fmt.Sprintf("The Blue Shell was thrown at %s, but their Moon illusion redirected it! %s lost %.0f points instead!", displayName, randomDisplayName, deductAmount),
				PointsDelta:       0,
				PoolDelta:         0,
				TargetUserID:      &randomUser.DiscordID,
				TargetPointsDelta: -deductAmount,
			}
			return nil
		}

		blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, firstPlaceUser.ID, guildID)
		if err != nil {
			return err
		}

		if blocked {
			firstPlaceUsername := firstPlaceUser.Username
			displayName := ""
			if firstPlaceUsername == nil || *firstPlaceUsername == "" {
				displayName = fmt.Sprintf("<@%s>", firstPlaceUser.DiscordID)
			} else {
				displayName = *firstPlaceUsername
			}

			result = &models.CardResult{
				Message:     fmt.Sprintf("The Blue Shell was thrown at %s, but their %s blocked it!", displayName, protectionName(blockedByRedshirt)),
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		percentage := 0.1
		deductAmount := firstPlaceUser.Points * percentage
		if firstPlaceUser.Points < deductAmount {
			deductAmount = firstPlaceUser.Points
		}

		firstPlaceUsername := firstPlaceUser.Username
		displayName := ""
		if firstPlaceUsername == nil || *firstPlaceUsername == "" {
			displayName = fmt.Sprintf("<@%s>", firstPlaceUser.DiscordID)
		} else {
			displayName = *firstPlaceUsername
		}

		result = &models.CardResult{
			Message:           fmt.Sprintf("The Blue Shell hit %s! They lost %.0f points.", displayName, deductAmount),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &firstPlaceUser.DiscordID,
			TargetPointsDelta: -deductAmount,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleNuke(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ? and deleted_at is null", guildID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found in the server. The Nuke fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	if err := db.Model(&models.User{}).
		Where("guild_id = ?", guildID).
		Update("points", gorm.Expr("points * 0.75")).Error; err != nil {
		return nil, err
	}

	var guild models.Guild
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("guild_id = ?", guildID).
		First(&guild).Error; err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     "You've drawn The Nuke! Everyone (including you) loses 25% of their points to the Pool.",
		PointsDelta: 0,
		PoolDelta:   -(guild.Pool * 0.25),
	}, nil
}

func handleEMP(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ? and deleted_at is null", guildID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found in the server. EMP fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	if err := db.Model(&models.User{}).
		Where("guild_id = ?", guildID).
		Update("points", gorm.Expr("points * 0.95")).Error; err != nil {
		return nil, err
	}

	var guild models.Guild
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("guild_id = ?", guildID).
		First(&guild).Error; err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     "You've drawn EMP! Everyone (including you and the pool) loses 5% of their points.",
		PointsDelta: 0,
		PoolDelta:   -(guild.Pool * 0.05),
	}, nil
}

func handleDivineIntervention(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ? and deleted_at is null", guildID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found in the server. Divine Intervention fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	averagePoints := 0.0
	for _, user := range allUsers {
		averagePoints += user.Points
	}
	averagePoints /= float64(len(allUsers))

	var user models.User
	db.First(&user, "discord_id = ? AND guild_id = ?", userID, guildID)
	if user.ID == 0 {
		return nil, fmt.Errorf("user not found")
	}

	if user.Points >= averagePoints {
		return &models.CardResult{
			Message:     "You are already at or above average! Divine Intervention fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	pointsDelta := averagePoints - user.Points

	return &models.CardResult{
		Message:     fmt.Sprintf("Your points balance is set to exactly the average of all players (%.2f).", averagePoints),
		PointsDelta: pointsDelta,
		PoolDelta:   0,
	}, nil
}

func handleWhale(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You've drawn The Whale! You gained 750 points immediately.",
		PointsDelta: 750,
		PoolDelta:   0,
	}, nil
}

func handleGuillotine(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	db.First(&user, "discord_id = ? AND guild_id = ?", userID, guildID)
	if user.ID == 0 {
		return nil, fmt.Errorf("user not found")
	}

	return &models.CardResult{
		Message:     "You've drawn The Guillotine! You lost 15% of your points.",
		PointsDelta: -(user.Points * 0.15),
		PoolDelta:   0,
	}, nil
}

func handleRobbingTheHood(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var poorestUser models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			Order("points ASC").
			First(&poorestUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				result = &models.CardResult{
					Message:     "No players found in the server. Robbing the Hood fizzles out.",
					PointsDelta: 0,
					PoolDelta:   0,
				}
				return nil
			}
			return err
		}

		if poorestUser.DiscordID == userID {
			result = &models.CardResult{
				Message:     "You're the poorest player! Robbing the Hood fizzles out.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		var drawer models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&drawer).Error; err != nil {
			return err
		}

		stolenAmount := poorestUser.Points * 0.1
		poorestDisplayName := ""
		if poorestUser.Username == nil || *poorestUser.Username == "" {
			poorestDisplayName = fmt.Sprintf("<@%s>", poorestUser.DiscordID)
		} else {
			poorestDisplayName = *poorestUser.Username
		}

		moonRedirected, err := CheckAndConsumeMoon(tx, poorestUser.ID, guildID)
		if err != nil {
			return err
		}
		if moonRedirected {
			randomUserDiscordID, err := GetRandomUserForMoon(tx, guildID, []uint{poorestUser.ID, drawer.ID})
			if err != nil {
				blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, poorestUser.ID, guildID)
				if err != nil {
					return err
				}
				if blocked {
					result = &models.CardResult{
						Message:     fmt.Sprintf("Robbing the Hood targeted %s, but their Moon illusion tried to redirect with no eligible users. %s blocked the theft! The card fizzles out.", poorestDisplayName, protectionName(blockedByRedshirt)),
						PointsDelta: 0,
						PoolDelta:   0,
					}
				} else {
					result = &models.CardResult{
						Message:     fmt.Sprintf("Robbing the Hood targeted %s, but their Moon illusion tried to redirect with no eligible users. The card fizzles out.", poorestDisplayName),
						PointsDelta: 0,
						PoolDelta:   0,
					}
				}
				return nil
			}
			var randomUser models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("discord_id = ? AND guild_id = ?", randomUserDiscordID, guildID).
				First(&randomUser).Error; err != nil {
				return err
			}
			redirectStolen := randomUser.Points * 0.1
			if redirectStolen > randomUser.Points {
				redirectStolen = randomUser.Points
			}
			randomUser.Points -= redirectStolen
			if randomUser.Points < 0 {
				randomUser.Points = 0
			}
			if err := tx.Save(&randomUser).Error; err != nil {
				return err
			}
			drawer.Points += redirectStolen
			if err := tx.Save(&drawer).Error; err != nil {
				return err
			}
			randomDisplayName := ""
			if randomUser.Username == nil || *randomUser.Username == "" {
				randomDisplayName = fmt.Sprintf("<@%s>", randomUser.DiscordID)
			} else {
				randomDisplayName = *randomUser.Username
			}
			result = &models.CardResult{
				Message:           fmt.Sprintf("Robbing the Hood targeted %s, but their Moon illusion redirected it! You stole 10%% of %s's points (%.0f) instead.", poorestDisplayName, randomDisplayName, redirectStolen),
				PointsDelta:       redirectStolen,
				PoolDelta:         0,
				TargetUserID:      &randomUser.DiscordID,
				TargetPointsDelta: -redirectStolen,
			}
			return nil
		}

		blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, poorestUser.ID, guildID)
		if err != nil {
			return err
		}
		if blocked {
			result = &models.CardResult{
				Message:     fmt.Sprintf("Robbing the Hood targeted %s, but their %s blocked the theft! The card fizzles out.", poorestDisplayName, protectionName(blockedByRedshirt)),
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		poorestUser.Points -= stolenAmount
		if err := tx.Save(&poorestUser).Error; err != nil {
			return err
		}
		drawer.Points += stolenAmount
		if err := tx.Save(&drawer).Error; err != nil {
			return err
		}

		result = &models.CardResult{
			Message:           "You've drawn Robbing the Hood! You stole 10% of the poorest player's points and gave it to yourself.",
			PointsDelta:       stolenAmount,
			PoolDelta:         0,
			TargetUserID:      &poorestUser.DiscordID,
			TargetPointsDelta: -stolenAmount,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleStopTheSteal(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     fmt.Sprintf("<@%s> drew STOP THE STEAL! Use /play-card to play this card and cancel any of your active bets.", userID),
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handlePoolBoy(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     fmt.Sprintf("<@%s> drew Pool Boy! Use /play-card to play this card and clean the algae from the pool.", userID),
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleSnipSnap(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		var entries []models.BetEntry
		if err := tx.Preload("Bet").
			Joins("JOIN bets ON bets.id = bet_entries.bet_id").
			Where("bet_entries.user_id = ? AND bets.paid = ? AND bet_entries.deleted_at IS NULL", user.ID, false).
			Find(&entries).Error; err != nil {
			return err
		}

		if len(entries) == 0 {
			result = &models.CardResult{
				Message:     "You have no active bets to flip. The card fizzles out.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		randomIndex := rand.Intn(len(entries))
		entryToFlip := entries[randomIndex]

		oldOption := entryToFlip.Option
		newOption := 0
		if oldOption == 1 {
			newOption = 2
		} else {
			newOption = 1
		}

		entryToFlip.Option = newOption
		if err := tx.Save(&entryToFlip).Error; err != nil {
			return err
		}

		betName := entryToFlip.Bet.Description
		newOptionName := ""
		if newOption == 1 {
			newOptionName = entryToFlip.Bet.Option1
		} else {
			newOptionName = entryToFlip.Bet.Option2
		}

		result = &models.CardResult{
			Message:     fmt.Sprintf("Snip Snap Snip Snap! Your bet on **%s** has been flipped! You are now betting on **%s**.", betName, newOptionName),
			PointsDelta: 0,
			PoolDelta:   0,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleGambler(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     fmt.Sprintf("<@%s> drew The Gambler! You must choose your fate...", userID),
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleGetOutOfJail(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     fmt.Sprintf("<@%s> drew Get Out of Jail Free! This card will nullify your next lost bet completely.", userID),
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleBankHeist(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	guild, err := guildService.GetGuildInfo(s, db, guildID, "")
	if err != nil {
		return nil, err
	}

	if guild.Pool < 300 {
		return &models.CardResult{
			Message:     fmt.Sprintf("You've drawn Bank Heist! You stole %0.f points from the pool.", guild.Pool),
			PointsDelta: guild.Pool,
			PoolDelta:   -guild.Pool,
		}, nil
	}

	return &models.CardResult{
		Message:     "You've drawn Bank Heist! You stole 300 points from the pool.",
		PointsDelta: 300,
		PoolDelta:   -300,
	}, nil
}

func handleLehmanBrothersInsider(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	guild, err := guildService.GetGuildInfo(s, db, guildID, "")
	if err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     "You've drawn Lehman Brothers Insider! The pool loses 20% of its total points.",
		PointsDelta: 0,
		PoolDelta:   -(guild.Pool * 0.2),
	}, nil
}

func handleInsiderTrading(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You've drawn Insider Trading! You stole 100 points from the pool.",
		PointsDelta: 100,
		PoolDelta:   -100,
	}, nil
}

func handleHotPotato(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		var allUsers []models.User
		if err := tx.Where("guild_id = ? AND discord_id != ? and deleted_at is null", guildID, userID).Find(&allUsers).Error; err != nil {
			return err
		}

		if len(allUsers) == 0 {
			result = &models.CardResult{
				Message:     "No other users found to pass the hot potato to. The card fizzles out.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		randomIndex := rand.Intn(len(allUsers))
		targetUser := allUsers[randomIndex]

		var lockedTarget models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", targetUser.DiscordID, guildID).
			First(&lockedTarget).Error; err != nil {
			return err
		}

		userLoss := 50.0
		targetLoss := 50.0

		if user.Points < userLoss {
			userLoss = user.Points
		}
		if lockedTarget.Points < targetLoss {
			targetLoss = lockedTarget.Points
		}

		userMention := "<@" + userID + ">"
		targetMention := "<@" + lockedTarget.DiscordID + ">"
		drawerActualLoss := 0.0
		targetActualLoss := 0.0
		var messageParts []string

		// Shield/moon for drawer (user)
		moonRedirected, err := CheckAndConsumeMoon(tx, user.ID, guildID)
		if err != nil {
			return err
		}
		if moonRedirected {
			randomDiscordID, err := GetRandomUserForMoon(tx, guildID, []uint{user.ID, lockedTarget.ID})
			if err != nil {
				shieldBlocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, user.ID, guildID)
				if err != nil {
					return err
				}
				if shieldBlocked {
					messageParts = append(messageParts, fmt.Sprintf("%s's Moon tried to redirect but no eligible users. %s blocked your loss!", userMention, protectionName(blockedByRedshirt)))
				} else {
					messageParts = append(messageParts, fmt.Sprintf("%s's Moon tried to redirect but no eligible users. Your loss fizzles.", userMention))
				}
			} else {
				var randomUser models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("discord_id = ? AND guild_id = ?", randomDiscordID, guildID).First(&randomUser).Error; err != nil {
					return err
				}
				deduct := userLoss
				if randomUser.Points < deduct {
					deduct = randomUser.Points
				}
				randomUser.Points -= deduct
				if randomUser.Points < 0 {
					randomUser.Points = 0
				}
				if err := tx.Save(&randomUser).Error; err != nil {
					return err
				}
				randomMention := "<@" + randomUser.DiscordID + ">"
				messageParts = append(messageParts, fmt.Sprintf("%s's Moon redirected your 50 loss to %s (%.0f points)!", userMention, randomMention, deduct))
			}
		} else {
			shieldBlocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, user.ID, guildID)
			if err != nil {
				return err
			}
			if shieldBlocked {
				drawerActualLoss = 0
				messageParts = append(messageParts, fmt.Sprintf("%s's %s blocked your loss!", userMention, protectionName(blockedByRedshirt)))
			} else {
				drawerActualLoss = userLoss
				user.Points -= userLoss
				if err := tx.Save(&user).Error; err != nil {
					return err
				}
			}
		}

		// Shield/moon for target
		moonRedirectedTarget, err := CheckAndConsumeMoon(tx, lockedTarget.ID, guildID)
		if err != nil {
			return err
		}
		if moonRedirectedTarget {
			randomDiscordID, err := GetRandomUserForMoon(tx, guildID, []uint{user.ID, lockedTarget.ID})
			if err != nil {
				shieldBlocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedTarget.ID, guildID)
				if err != nil {
					return err
				}
				if shieldBlocked {
					messageParts = append(messageParts, fmt.Sprintf("%s's Moon tried to redirect but no eligible users. %s blocked their loss!", targetMention, protectionName(blockedByRedshirt)))
				} else {
					messageParts = append(messageParts, fmt.Sprintf("%s's Moon tried to redirect but no eligible users. Their loss fizzles.", targetMention))
				}
			} else {
				var randomUser models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("discord_id = ? AND guild_id = ?", randomDiscordID, guildID).First(&randomUser).Error; err != nil {
					return err
				}
				deduct := targetLoss
				if randomUser.Points < deduct {
					deduct = randomUser.Points
				}
				randomUser.Points -= deduct
				if randomUser.Points < 0 {
					randomUser.Points = 0
				}
				if err := tx.Save(&randomUser).Error; err != nil {
					return err
				}
				randomMention := "<@" + randomUser.DiscordID + ">"
				messageParts = append(messageParts, fmt.Sprintf("%s's Moon redirected the hit to %s (%.0f points)!", targetMention, randomMention, deduct))
			}
		} else {
			shieldBlocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, lockedTarget.ID, guildID)
			if err != nil {
				return err
			}
			if shieldBlocked {
				targetActualLoss = 0
				messageParts = append(messageParts, fmt.Sprintf("%s's %s blocked the hot potato!", targetMention, protectionName(blockedByRedshirt)))
			} else {
				targetActualLoss = targetLoss
				lockedTarget.Points -= targetLoss
				if err := tx.Save(&lockedTarget).Error; err != nil {
					return err
				}
			}
		}

		msg := "🥔 Hot Potato! "
		if len(messageParts) > 0 {
			msg += strings.Join(messageParts, " ")
		}
		if drawerActualLoss > 0 || targetActualLoss > 0 {
			if drawerActualLoss > 0 && targetActualLoss > 0 {
				msg += fmt.Sprintf("%s lost %.0f points. The hot potato was passed to %s who also lost %.0f points!", userMention, drawerActualLoss, targetMention, targetActualLoss)
			} else if drawerActualLoss > 0 {
				msg += fmt.Sprintf("%s lost %.0f points. The hot potato was passed to %s who was protected.", userMention, drawerActualLoss, targetMention)
			} else if targetActualLoss > 0 {
				msg += fmt.Sprintf("You were protected. The hot potato was passed to %s who lost %.0f points!", targetMention, targetActualLoss)
			}
		}

		targetID := lockedTarget.DiscordID
		result = &models.CardResult{
			Message:           msg,
			PointsDelta:       -drawerActualLoss,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: -targetActualLoss,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleDuel(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "DUEL! requires you to select a target!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecuteDuel(db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		var targetUser models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
			First(&targetUser).Error; err != nil {
			return err
		}

		userRoll := rand.Intn(6) + 1
		targetRoll := rand.Intn(6) + 1

		targetID := targetUserID
		targetMention := "<@" + targetUserID + ">"
		userMention := "<@" + userID + ">"

		if userRoll > targetRoll {
			transferAmount := 100.0
			if targetUser.Points < transferAmount {
				transferAmount = targetUser.Points
			}

			// Shield/moon for loser (target)
			moonRedirected, err := CheckAndConsumeMoon(tx, targetUser.ID, guildID)
			if err != nil {
				return err
			}
			if moonRedirected {
				randomDiscordID, err := GetRandomUserForMoon(tx, guildID, []uint{targetUser.ID, user.ID})
				if err != nil {
					blocked, blockedByRedshirt, _ := CheckAndConsumeShieldOrRedshirt(tx, targetUser.ID, guildID)
					if blocked {
						result = &models.CardResult{
							Message:           fmt.Sprintf("⚔️ DUEL! %s rolled %d, %s rolled %d. You win! But %s's Moon had no one to redirect to and their %s blocked the loss! The duel fizzles.", userMention, userRoll, targetMention, targetRoll, targetMention, protectionName(blockedByRedshirt)),
							PointsDelta:       0,
							PoolDelta:         0,
							TargetUserID:      &targetID,
							TargetPointsDelta: 0,
						}
					} else {
						result = &models.CardResult{
							Message:           fmt.Sprintf("⚔️ DUEL! %s rolled %d, %s rolled %d. You win! But %s's Moon had no one to redirect to. The duel fizzles.", userMention, userRoll, targetMention, targetRoll, targetMention),
							PointsDelta:       0,
							PoolDelta:         0,
							TargetUserID:      &targetID,
							TargetPointsDelta: 0,
						}
					}
					return nil
				}
				var randomUser models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("discord_id = ? AND guild_id = ?", randomDiscordID, guildID).First(&randomUser).Error; err != nil {
					return err
				}
				actualTransfer := transferAmount
				if randomUser.Points < actualTransfer {
					actualTransfer = randomUser.Points
				}
				randomUser.Points -= actualTransfer
				if randomUser.Points < 0 {
					randomUser.Points = 0
				}
				tx.Save(&randomUser)
				user.Points += actualTransfer
				tx.Save(&user)
				randomMention := "<@" + randomUser.DiscordID + ">"
				result = &models.CardResult{
					Message:           fmt.Sprintf("⚔️ DUEL! %s rolled %d, %s rolled %d. You win! %s's Moon redirected the loss to %s — you gained %.0f points!", userMention, userRoll, targetMention, targetRoll, targetMention, randomMention, actualTransfer),
					PointsDelta:       actualTransfer,
					PoolDelta:         0,
					TargetUserID:      &targetID,
					TargetPointsDelta: 0,
				}
				return nil
			}
			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, targetUser.ID, guildID)
			if err != nil {
				return err
			}
			if blocked {
				result = &models.CardResult{
					Message:           fmt.Sprintf("⚔️ DUEL! %s rolled %d, %s rolled %d. You win! But %s's %s blocked the loss! The duel fizzles.", userMention, userRoll, targetMention, targetRoll, targetMention, protectionName(blockedByRedshirt)),
					PointsDelta:       0,
					PoolDelta:         0,
					TargetUserID:      &targetID,
					TargetPointsDelta: 0,
				}
				return nil
			}

			user.Points += transferAmount
			targetUser.Points -= transferAmount

			if err := tx.Save(&user).Error; err != nil {
				return err
			}
			if err := tx.Save(&targetUser).Error; err != nil {
				return err
			}

			result = &models.CardResult{
				Message:           fmt.Sprintf("⚔️ DUEL! %s rolled %d, %s rolled %d. You win! You gained %.0f points from %s!", userMention, userRoll, targetMention, targetRoll, transferAmount, targetMention),
				PointsDelta:       transferAmount,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: -transferAmount,
			}
		} else if targetRoll > userRoll {
			transferAmount := 100.0
			if user.Points < transferAmount {
				transferAmount = user.Points
			}

			moonRedirected, err := CheckAndConsumeMoon(tx, user.ID, guildID)
			if err != nil {
				return err
			}
			if moonRedirected {
				randomDiscordID, err := GetRandomUserForMoon(tx, guildID, []uint{user.ID, targetUser.ID})
				if err != nil {
					blocked, blockedByRedshirt, _ := CheckAndConsumeShieldOrRedshirt(tx, user.ID, guildID)
					if blocked {
						result = &models.CardResult{
							Message:           fmt.Sprintf("⚔️ DUEL! %s rolled %d, %s rolled %d. You lose! Your Moon had no one to redirect to and your %s blocked the loss! The duel fizzles.", userMention, userRoll, targetMention, targetRoll, protectionName(blockedByRedshirt)),
							PointsDelta:       0,
							PoolDelta:         0,
							TargetUserID:      &targetID,
							TargetPointsDelta: 0,
						}
					} else {
						result = &models.CardResult{
							Message:           fmt.Sprintf("⚔️ DUEL! %s rolled %d, %s rolled %d. You lose! Your Moon had no one to redirect to. The duel fizzles.", userMention, userRoll, targetMention, targetRoll),
							PointsDelta:       0,
							PoolDelta:         0,
							TargetUserID:      &targetID,
							TargetPointsDelta: 0,
						}
					}
					return nil
				}
				var randomUser models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("discord_id = ? AND guild_id = ?", randomDiscordID, guildID).First(&randomUser).Error; err != nil {
					return err
				}
				actualTransfer := transferAmount
				if randomUser.Points < actualTransfer {
					actualTransfer = randomUser.Points
				}
				randomUser.Points -= actualTransfer
				if randomUser.Points < 0 {
					randomUser.Points = 0
				}
				tx.Save(&randomUser)
				targetUser.Points += actualTransfer
				tx.Save(&targetUser)
				randomMention := "<@" + randomUser.DiscordID + ">"
				result = &models.CardResult{
					Message:           fmt.Sprintf("⚔️ DUEL! %s rolled %d, %s rolled %d. You lose! Your Moon redirected the loss to %s — %s gained %.0f points!", userMention, userRoll, targetMention, targetRoll, randomMention, targetMention, actualTransfer),
					PointsDelta:       0,
					PoolDelta:         0,
					TargetUserID:      &targetID,
					TargetPointsDelta: actualTransfer,
				}
				return nil
			}
			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(tx, user.ID, guildID)
			if err != nil {
				return err
			}
			if blocked {
				result = &models.CardResult{
					Message:           fmt.Sprintf("⚔️ DUEL! %s rolled %d, %s rolled %d. You lose! But your %s blocked the loss! The duel fizzles.", userMention, userRoll, targetMention, targetRoll, protectionName(blockedByRedshirt)),
					PointsDelta:       0,
					PoolDelta:         0,
					TargetUserID:      &targetID,
					TargetPointsDelta: 0,
				}
				return nil
			}

			user.Points -= transferAmount
			targetUser.Points += transferAmount

			if err := tx.Save(&user).Error; err != nil {
				return err
			}
			if err := tx.Save(&targetUser).Error; err != nil {
				return err
			}

			result = &models.CardResult{
				Message:           fmt.Sprintf("⚔️ DUEL! %s rolled %d, %s rolled %d. You lose! %s gained %.0f points from you!", userMention, userRoll, targetMention, targetRoll, targetMention, transferAmount),
				PointsDelta:       -transferAmount,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: transferAmount,
			}
		} else {
			userLoss := 50.0
			targetLoss := 50.0

			if user.Points < userLoss {
				userLoss = user.Points
			}
			if targetUser.Points < targetLoss {
				targetLoss = targetUser.Points
			}

			userActualLoss := 0.0
			targetActualLoss := 0.0

			moonU, _ := CheckAndConsumeMoon(tx, user.ID, guildID)
			if moonU {
				randomID, err := GetRandomUserForMoon(tx, guildID, []uint{user.ID, targetUser.ID})
				if err != nil {
					CheckAndConsumeShieldOrRedshirt(tx, user.ID, guildID)
				} else {
					var ru models.User
					tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("discord_id = ? AND guild_id = ?", randomID, guildID).First(&ru)
					deduct := userLoss
					if ru.Points < deduct {
						deduct = ru.Points
					}
					ru.Points -= deduct
					if ru.Points < 0 {
						ru.Points = 0
					}
					tx.Save(&ru)
				}
			} else {
				if blocked, _, _ := CheckAndConsumeShieldOrRedshirt(tx, user.ID, guildID); !blocked {
					userActualLoss = userLoss
					user.Points -= userLoss
					tx.Save(&user)
				}
			}

			moonT, _ := CheckAndConsumeMoon(tx, targetUser.ID, guildID)
			if moonT {
				randomID, err := GetRandomUserForMoon(tx, guildID, []uint{user.ID, targetUser.ID})
				if err != nil {
					CheckAndConsumeShieldOrRedshirt(tx, targetUser.ID, guildID)
				} else {
					var ru models.User
					tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("discord_id = ? AND guild_id = ?", randomID, guildID).First(&ru)
					deduct := targetLoss
					if ru.Points < deduct {
						deduct = ru.Points
					}
					ru.Points -= deduct
					if ru.Points < 0 {
						ru.Points = 0
					}
					tx.Save(&ru)
				}
			} else {
				if blocked, _, _ := CheckAndConsumeShieldOrRedshirt(tx, targetUser.ID, guildID); !blocked {
					targetActualLoss = targetLoss
					targetUser.Points -= targetLoss
					tx.Save(&targetUser)
				}
			}

			result = &models.CardResult{
				Message:           fmt.Sprintf("⚔️ DUEL! You both rolled %d! It's a tie! You lost %.0f points, %s lost %.0f points.", userRoll, userActualLoss, targetMention, targetActualLoss),
				PointsDelta:       -userActualLoss,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: -targetActualLoss,
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleTag(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Tag! requires you to select a target user!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecuteTag(db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var targetUser models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).First(&targetUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &models.CardResult{
				Message:     "Target user not found in this server.",
				PointsDelta: 0,
				PoolDelta:   0,
			}, nil
		}
		return nil, err
	}

	inventory := models.UserInventory{
		UserID:  targetUser.ID,
		GuildID: guildID,
		CardID:  TagCardID,
	}
	if err := db.Create(&inventory).Error; err != nil {
		return nil, err
	}

	targetID := targetUserID
	targetMention := "<@" + targetUserID + ">"
	userMention := "<@" + userID + ">"

	return &models.CardResult{
		Message:           fmt.Sprintf("🏷️ Tag! %s tagged %s! %s will gain 1 point every time anyone buys a card for the next 12 hours.", userMention, targetMention, targetMention),
		PointsDelta:       0,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: 0,
	}, nil
}

func handleCrowdfund(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ? and deleted_at is null", guildID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	pointsDelta := float64(len(allUsers))

	return &models.CardResult{
		Message:     "You've drawn Crowdfund! You get 1 point for every user in the server.",
		PointsDelta: pointsDelta,
		PoolDelta:   0,
	}, nil
}

func handleReversePickpocket(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		var allUsers []models.User
		if err := tx.Where("guild_id = ? AND discord_id != ? and deleted_at is null", guildID, userID).Find(&allUsers).Error; err != nil {
			return err
		}

		if len(allUsers) == 0 {
			result = &models.CardResult{
				Message:     "No other users found to give points to. The card fizzles out.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		randomIndex := rand.Intn(len(allUsers))
		targetUser := allUsers[randomIndex]

		var lockedTarget models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", targetUser.DiscordID, guildID).
			First(&lockedTarget).Error; err != nil {
			return err
		}

		transferAmount := 150.0
		if user.Points < transferAmount {
			transferAmount = user.Points
		}

		user.Points -= transferAmount
		lockedTarget.Points += transferAmount

		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		if err := tx.Save(&lockedTarget).Error; err != nil {
			return err
		}

		targetID := lockedTarget.DiscordID
		targetMention := "<@" + lockedTarget.DiscordID + ">"
		userMention := "<@" + userID + ">"

		result = &models.CardResult{
			Message:           fmt.Sprintf("🫴 Reverse Pickpocket! %s sneakily gave %.0f points to %s!", userMention, transferAmount, targetMention),
			PointsDelta:       -transferAmount,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: transferAmount,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleShoppingSpree(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
			return err
		}

		now := time.Now()
		expirationTime := now.Add(-12 * time.Hour)

		var allShoppingSpreeCards []models.UserInventory
		if err := tx.Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL", user.ID, guildID, ShoppingSpreeCardID).
			Order("created_at ASC").
			Find(&allShoppingSpreeCards).Error; err != nil {
			return err
		}

		var activeCards []models.UserInventory
		for _, card := range allShoppingSpreeCards {
			if card.CreatedAt.After(expirationTime) || card.CreatedAt.Equal(expirationTime) {
				activeCards = append(activeCards, card)
			}
		}

		if len(activeCards) > 1 {
			for i := 1; i < len(activeCards); i++ {
				if err := tx.Delete(&activeCards[i]).Error; err != nil {
					return err
				}
			}
			result = &models.CardResult{
				Message:     "You already have an active Shopping Spree! This card fizzles out.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
		} else {
			result = &models.CardResult{
				Message:     "Shopping Spree activated! Your card-buying costs are reduced by 50% for 12 hours.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleBountyHunter(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Bounty Hunter requires you to select a target user!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecuteBountyHunter(db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var targetUser models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).First(&targetUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &models.CardResult{
				Message:     "Target user not found in this server.",
				PointsDelta: 0,
				PoolDelta:   0,
			}, nil
		}
		return nil, err
	}

	inventory := models.UserInventory{
		UserID:  targetUser.ID,
		GuildID: guildID,
		CardID:  BountyHunterCardID,
	}
	if err := db.Create(&inventory).Error; err != nil {
		return nil, err
	}

	targetID := targetUserID
	targetMention := "<@" + targetUserID + ">"
	userMention := "<@" + userID + ">"

	return &models.CardResult{
		Message:           fmt.Sprintf("🎯 Bounty Hunter! %s placed a bounty on %s! The next person to steal from them will collect 100 points from the pool!", userMention, targetMention),
		PointsDelta:       0,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: 0,
	}, nil
}

func handleSocialDistancing(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Social Distancing requires you to select a target user!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecuteSocialDistancing(db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var targetUser models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
		First(&targetUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &models.CardResult{
				Message:     "Target user not found in this server.",
				PointsDelta: 0,
				PoolDelta:   0,
			}, nil
		}
		return nil, err
	}
	targetMention := "<@" + targetUserID + ">"

	moonRedirected, err := CheckAndConsumeMoon(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if moonRedirected {
		randomUserID, err := GetRandomUserForMoon(db, guildID, []uint{targetUser.ID})
		if err != nil {
			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, targetUser.ID, guildID)
			if err != nil {
				return nil, err
			}
			if blocked {
				targetID := targetUserID
				return &models.CardResult{
					Message:           fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. %s blocked the Social Distancing instead!", targetMention, protectionName(blockedByRedshirt)),
					PointsDelta:       0,
					PoolDelta:         0,
					TargetUserID:      &targetID,
					TargetPointsDelta: 0,
				}, nil
			}
			spareKeyBlocked, err := checkAndConsumeSpareKey(db, targetUser.ID, guildID)
			if err != nil {
				return nil, err
			}
			if spareKeyBlocked {
				targetID := targetUserID
				return &models.CardResult{
					Message:           fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. Spare Key blocked the Social Distancing instead!", targetMention),
					PointsDelta:       0,
					PoolDelta:         0,
					TargetUserID:      &targetID,
					TargetPointsDelta: 0,
				}, nil
			}
			targetID := targetUserID
			return &models.CardResult{
				Message:           fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. The distancing fizzles out.", targetMention),
				PointsDelta:       0,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: 0,
			}, nil
		}

		var randomUser models.User
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", randomUserID, guildID).
			First(&randomUser).Error; err != nil {
			return nil, err
		}

		lockoutUntil := time.Now().Add(2 * time.Hour)
		randomUser.CardDrawTimeoutUntil = &lockoutUntil
		if err := db.Save(&randomUser).Error; err != nil {
			return nil, err
		}

		inventory := models.UserInventory{
			UserID:  randomUser.ID,
			GuildID: guildID,
			CardID:  ShieldCardID,
		}
		if err := db.Create(&inventory).Error; err != nil {
			return nil, err
		}

		randomMention := "<@" + randomUserID + ">"
		targetID := randomUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's Moon illusion redirected the Social Distancing! %s received a Shield and cannot buy cards for 2 hours instead!", targetMention, randomMention),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: 0,
		}, nil
	}

	blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if blocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's %s blocked the Social Distancing!", targetMention, protectionName(blockedByRedshirt)),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: 0,
		}, nil
	}

	spareKeyBlocked, err := checkAndConsumeSpareKey(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if spareKeyBlocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's Spare Key got them out of Social Distancing!", targetMention),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: 0,
		}, nil
	}

	inventory := models.UserInventory{
		UserID:  targetUser.ID,
		GuildID: guildID,
		CardID:  ShieldCardID,
	}
	if err := db.Create(&inventory).Error; err != nil {
		return nil, err
	}

	lockoutUntil := time.Now().Add(2 * time.Hour)
	targetUser.CardDrawTimeoutUntil = &lockoutUntil

	if err := db.Save(&targetUser).Error; err != nil {
		return nil, err
	}

	targetID := targetUserID
	userMention := "<@" + userID + ">"

	return &models.CardResult{
		Message:           fmt.Sprintf("Social Distancing! %s gave %s a Shield, but %s cannot buy any new cards for 2 hours.", userMention, targetMention, targetMention),
		PointsDelta:       0,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: 0,
	}, nil
}

func handleTheHermit(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", userID, guildID).
		First(&user).Error; err != nil {
		return nil, err
	}

	moonRedirected, err := CheckAndConsumeMoon(db, user.ID, guildID)
	if err != nil {
		return nil, err
	}
	if moonRedirected {
		randomUserID, err := GetRandomUserForMoon(db, guildID, []uint{user.ID})
		if err != nil {
			var shieldBlocked, blockedByRedshirt, spareKeyBlocked bool
			shieldBlocked, blockedByRedshirt, err = CheckAndConsumeShieldOrRedshirt(db, user.ID, guildID)
			if err != nil {
				return nil, err
			}
			if !shieldBlocked {
				spareKeyBlocked, err = checkAndConsumeSpareKey(db, user.ID, guildID)
				if err != nil {
					return nil, err
				}
			}

			inventory := models.UserInventory{
				UserID:  user.ID,
				GuildID: guildID,
				CardID:  ShieldCardID,
			}
			if err := db.Create(&inventory).Error; err != nil {
				return nil, err
			}

			var message string
			if shieldBlocked {
				message = fmt.Sprintf("You went into seclusion and gained a Shield! Your Moon illusion tried to redirect, but no eligible users found. Your existing %s protected you from the timeout, so you can still buy cards.", protectionName(blockedByRedshirt))
			} else if spareKeyBlocked {
				message = "You went into seclusion and gained a Shield! Your Moon illusion tried to redirect, but no eligible users found. Your Spare Key protected you from the timeout, so you can still buy cards."
			} else {
				lockoutUntil := time.Now().Add(12 * time.Hour)
				user.CardDrawTimeoutUntil = &lockoutUntil

				if err := db.Save(&user).Error; err != nil {
					return nil, err
				}

				message = "You went into seclusion and gained a Shield! Your Moon illusion tried to redirect, but no eligible users found. However, you cannot buy any new cards for 12 hours."
			}

			return &models.CardResult{
				Message:     message,
				PointsDelta: 0,
				PoolDelta:   0,
			}, nil
		}

		var randomUser models.User
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", randomUserID, guildID).
			First(&randomUser).Error; err != nil {
			return nil, err
		}

		lockoutUntil := time.Now().Add(12 * time.Hour)
		randomUser.CardDrawTimeoutUntil = &lockoutUntil
		if err := db.Save(&randomUser).Error; err != nil {
			return nil, err
		}

		inventory := models.UserInventory{
			UserID:  randomUser.ID,
			GuildID: guildID,
			CardID:  ShieldCardID,
		}
		if err := db.Create(&inventory).Error; err != nil {
			return nil, err
		}

		randomMention := "<@" + randomUserID + ">"
		return &models.CardResult{
			Message:           fmt.Sprintf("You went into seclusion and gained a Shield! Your Moon illusion redirected the timeout! %s cannot buy any new cards for 12 hours instead!", randomMention),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &randomUserID,
			TargetPointsDelta: 0,
		}, nil
	}

	var shieldBlocked, blockedByRedshirt, spareKeyBlocked bool
	shieldBlocked, blockedByRedshirt, err = CheckAndConsumeShieldOrRedshirt(db, user.ID, guildID)
	if err != nil {
		return nil, err
	}
	if !shieldBlocked {
		spareKeyBlocked, err = checkAndConsumeSpareKey(db, user.ID, guildID)
		if err != nil {
			return nil, err
		}
	}

	inventory := models.UserInventory{
		UserID:  user.ID,
		GuildID: guildID,
		CardID:  ShieldCardID,
	}
	if err := db.Create(&inventory).Error; err != nil {
		return nil, err
	}

	var message string
	if shieldBlocked {
		message = fmt.Sprintf("You went into seclusion and gained a Shield! Your existing %s protected you from the timeout, so you can still buy cards.", protectionName(blockedByRedshirt))
	} else if spareKeyBlocked {
		message = "You went into seclusion and gained a Shield! Your Spare Key protected you from the timeout, so you can still buy cards."
	} else {
		lockoutUntil := time.Now().Add(12 * time.Hour)
		user.CardDrawTimeoutUntil = &lockoutUntil

		if err := db.Save(&user).Error; err != nil {
			return nil, err
		}

		message = "You went into seclusion and gained a Shield! However, you cannot buy any new cards for 12 hours."
	}

	return &models.CardResult{
		Message:     message,
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleStrength(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	inventory := models.UserInventory{
		UserID:  user.ID,
		GuildID: guildID,
		CardID:  ShieldCardID,
	}
	if err := db.Create(&inventory).Error; err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     "You channel your inner strength! You gained a Shield to block the next 'Steal' or negative effect played against you.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleTheHighPriestess(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "The High Priestess requires you to select a target user!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecuteTheHighPriestess(s *discordgo.Session, db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	targetID := targetUserID
	return &models.CardResult{
		Message:           "The High Priestess reveals hidden knowledge. Check your ephemeral message for the target user's inventory.",
		PointsDelta:       0,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: 0,
	}, nil
}

func handleTheEmpress(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var poorestUser models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ? AND deleted_at IS NULL", guildID).
			Order("points ASC").
			First(&poorestUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				result = &models.CardResult{
					Message:     "No players found in the server. The Empress has no one to bless.",
					PointsDelta: 0,
					PoolDelta:   0,
				}
				return nil
			}
			return err
		}

		gainAmount := 200.0
		poorestUserMention := "<@" + poorestUser.DiscordID + ">"
		result = &models.CardResult{
			Message:           fmt.Sprintf("The Empress blesses the poor! %s (in last place) gained %.0f points.", poorestUserMention, gainAmount),
			PointsDelta:       0,
			PoolDelta:         -gainAmount,
			TargetUserID:      &poorestUser.DiscordID,
			TargetPointsDelta: gainAmount,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleTheChariot(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "The Chariot grants you momentum! Your next 3 card draws will cost 0 points.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleJustice(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Justice requires you to select a target user within 500 points of you!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecuteJustice(s *discordgo.Session, db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var drawer models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", userID, guildID).
		First(&drawer).Error; err != nil {
		return nil, err
	}

	var target models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
		First(&target).Error; err != nil {
		return nil, err
	}

	targetMention := "<@" + targetUserID + ">"

	moonRedirected, err := CheckAndConsumeMoon(db, target.ID, guildID)
	if err != nil {
		return nil, err
	}
	if moonRedirected {
		randomUserID, err := GetRandomUserForMoon(db, guildID, []uint{target.ID, drawer.ID})
		if err != nil {
			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, target.ID, guildID)
			if err != nil {
				return nil, err
			}
			if blocked {
				targetID := targetUserID
				return &models.CardResult{
					Message:           fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. %s blocked Justice instead!", targetMention, protectionName(blockedByRedshirt)),
					PointsDelta:       0,
					PoolDelta:         0,
					TargetUserID:      &targetID,
					TargetPointsDelta: 0,
				}, nil
			}
			targetID := targetUserID
			return &models.CardResult{
				Message:           fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. Justice fizzles.", targetMention),
				PointsDelta:       0,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: 0,
			}, nil
		}

		var randomUser models.User
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", randomUserID, guildID).
			First(&randomUser).Error; err != nil {
			return nil, err
		}

		drawerOriginalPoints := drawer.Points
		randomOriginalPoints := randomUser.Points
		averagePoints := (drawer.Points + randomUser.Points) / 2.0
		drawer.Points = averagePoints
		randomUser.Points = averagePoints

		if err := db.Save(&drawer).Error; err != nil {
			return nil, err
		}
		if err := db.Save(&randomUser).Error; err != nil {
			return nil, err
		}

		randomMention := "<@" + randomUserID + ">"
		targetID := randomUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's Moon illusion redirected Justice! You and %s now have %.1f points (the average of your previous balances).", targetMention, randomMention, averagePoints),
			PointsDelta:       averagePoints - drawerOriginalPoints,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: averagePoints - randomOriginalPoints,
		}, nil
	}

	blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, target.ID, guildID)
	if err != nil {
		return nil, err
	}
	if blocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's %s blocked Justice!", targetMention, protectionName(blockedByRedshirt)),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: 0,
		}, nil
	}

	drawerOriginalPoints := drawer.Points
	targetOriginalPoints := target.Points
	averagePoints := (drawer.Points + target.Points) / 2.0
	drawer.Points = averagePoints
	target.Points = averagePoints

	if err := db.Save(&drawer).Error; err != nil {
		return nil, err
	}
	if err := db.Save(&target).Error; err != nil {
		return nil, err
	}

	targetID := targetUserID
	return &models.CardResult{
		Message:           fmt.Sprintf("Justice is served! Both you and %s now have %.1f points (the average of your previous balances).", targetMention, averagePoints),
		PointsDelta:       averagePoints - drawerOriginalPoints,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: averagePoints - targetOriginalPoints,
	}, nil
}

func handleTheHangedMan(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		lossAmount := 150.0
		if user.Points < lossAmount {
			lossAmount = user.Points
		}

		user.Points -= lossAmount
		if user.Points < 0 {
			user.Points = 0
		}

		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		result = &models.CardResult{
			Message:     fmt.Sprintf("The Hanged Man! You lost %.0f points immediately. In 24 hours, you'll gain 400 points from the pool.", lossAmount),
			PointsDelta: 0,
			PoolDelta:   0,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleTheStar(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ? AND deleted_at IS NULL", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found in the server. The Star has no one to bless.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	userPosition := 0
	for i, u := range allUsers {
		if u.ID == user.ID {
			userPosition = i + 1
			break
		}
	}

	if userPosition == 0 {
		return &models.CardResult{
			Message:     "Could not determine your position. The Star fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	totalPlayers := len(allUsers)
	bottom25PercentThreshold := totalPlayers - (totalPlayers / 4)
	if totalPlayers%4 != 0 {
		bottom25PercentThreshold = totalPlayers - ((totalPlayers + 3) / 4)
	}

	isBottom25Percent := userPosition > bottom25PercentThreshold

	if isBottom25Percent {
		return &models.CardResult{
			Message:     "The Star shines upon you! You're in the bottom 25% of players. +300 Points.",
			PointsDelta: 300,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:     "The Star sees you're doing well. Nothing happens.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleUnderdog(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var guild models.Guild
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return nil, err
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ? AND deleted_at IS NULL", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found in the server. The Underdog fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	userPosition := 0
	for i, u := range allUsers {
		if u.ID == user.ID {
			userPosition = i + 1
			break
		}
	}

	if userPosition == 0 {
		return &models.CardResult{
			Message:     "Could not determine your position. The Underdog fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	totalPlayers := len(allUsers)
	bottom25PercentThreshold := totalPlayers - (totalPlayers / 4)
	if totalPlayers%4 != 0 {
		bottom25PercentThreshold = totalPlayers - ((totalPlayers + 3) / 4)
	}
	isBottom25Percent := userPosition > bottom25PercentThreshold

	gainAmount := 50.0
	if isBottom25Percent {
		gainAmount = 250.0
	}
	if guild.Pool < gainAmount {
		gainAmount = guild.Pool
	}

	if gainAmount <= 0 {
		return &models.CardResult{
			Message:     "The pool is empty. The Underdog has nothing to give.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	if isBottom25Percent {
		return &models.CardResult{
			Message:     fmt.Sprintf("The Underdog! You're in the bottom 25%% of players. You gain %.0f points from the pool!", gainAmount),
			PointsDelta: gainAmount,
			PoolDelta:   -gainAmount,
		}, nil
	}
	return &models.CardResult{
		Message:     fmt.Sprintf("The Underdog! You're not in the bottom 25%%. You gain %.0f points from the pool.", gainAmount),
		PointsDelta: gainAmount,
		PoolDelta:   -gainAmount,
	}, nil
}

func handleHeismanCampaign(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ? AND deleted_at IS NULL", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found. Heisman Campaign fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	userPosition := 0
	for i, u := range allUsers {
		if u.ID == user.ID {
			userPosition = i + 1
			break
		}
	}

	if userPosition == 0 {
		return &models.CardResult{
			Message:     "Could not determine your position. Heisman Campaign fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	if userPosition <= 5 {
		inv := models.UserInventory{
			UserID:    user.ID,
			GuildID:   guildID,
			CardID:    HeismanCampaignCardID,
			CardCode:  "HCAM",
			ExpiresAt: nil,
		}
		if err := db.Create(&inv).Error; err != nil {
			return nil, err
		}
		return &models.CardResult{
			Message:     "Heisman Campaign! You're in the top 5. You gain 300 points. Your next bet win payout will be reduced by 15%.",
			PointsDelta: 300,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:     "Heisman Campaign! You're not in the top 5. You gain 100 points.",
		PointsDelta: 100,
		PoolDelta:   0,
	}, nil
}

func handleTheLovers(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "The Lovers requires you to select a target user!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecuteTheLovers(db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var targetUser models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).First(&targetUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &models.CardResult{
				Message:     "Target user not found in this server.",
				PointsDelta: 0,
				PoolDelta:   0,
			}, nil
		}
		return nil, err
	}

	inventory := models.UserInventory{
		UserID:       user.ID,
		GuildID:      guildID,
		CardID:       TheLoversCardID,
		TargetUserID: &targetUserID,
	}
	if err := db.Create(&inventory).Error; err != nil {
		return nil, err
	}

	targetID := targetUserID
	targetMention := "<@" + targetUserID + ">"
	userMention := "<@" + userID + ">"

	return &models.CardResult{
		Message:           fmt.Sprintf("💕 The Lovers! %s chose %s. For the next 24 hours, %s will earn 25%% of every bet won by %s!", userMention, targetMention, userMention, targetMention),
		PointsDelta:       0,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: 0,
	}, nil
}

// func handleMarketCrash(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
// 	var result *models.CardResult
// 	if err := db.Transaction(func(tx *gorm.DB) error {
// 		var activeBets []models.Bet
// 		if err := tx.Where("guild_id = ? AND paid = ?", guildID, false).Find(&activeBets).Error; err != nil {
// 			return err
// 		}

// 		if len(activeBets) == 0 {
// 			result = &models.CardResult{
// 				Message:     "Market Crash! No active bets to cancel.",
// 				PointsDelta: 0,
// 				PoolDelta:   0,
// 			}
// 			return nil
// 		}

// 		betIDs := make([]uint, len(activeBets))
// 		for i, bet := range activeBets {
// 			betIDs[i] = bet.ID
// 		}

// 		var entries []models.BetEntry
// 		if err := tx.Where("bet_id IN ?", betIDs).Find(&entries).Error; err != nil {
// 			return err
// 		}

// 		totalPoolDelta := 0.0
// 		for _, entry := range entries {
// 			totalPoolDelta += float64(entry.Amount)
// 		}

// 		if err := tx.Model(&models.Bet{}).
// 			Where("id IN ?", betIDs).
// 			Updates(map[string]interface{}{
// 				"active": false,
// 				"paid":   true,
// 			}).Error; err != nil {
// 			return err
// 		}

// 		if err := tx.Where("bet_id IN ?", betIDs).Delete(&models.BetEntry{}).Error; err != nil {
// 			return err
// 		}

// 		result = &models.CardResult{
// 			Message:     fmt.Sprintf("Market Crash! All active bets currently placed are cancelled; All money bet on them goes to the Pool. %.1f points have been added to the pool.", totalPoolDelta),
// 			PointsDelta: 0,
// 			PoolDelta:   totalPoolDelta,
// 		}
// 		return nil
// 	}); err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

func handleBlackHole(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var guild models.Guild
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			First(&guild).Error; err != nil {
			return err
		}

		var allUsers []models.User
		if err := tx.Where("guild_id = ? and deleted_at is null", guildID).Order("points ASC").Find(&allUsers).Error; err != nil {
			return err
		}

		if len(allUsers) == 0 {
			result = &models.CardResult{
				Message:     "No players found in the server. The Black Hole has no one to distribute to.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		poolAmount := guild.Pool * 0.25
		if poolAmount <= 0 {
			result = &models.CardResult{
				Message:     "The pool is empty. The Black Hole has nothing to distribute.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		numBottomPlayers := 5
		if len(allUsers) < numBottomPlayers {
			numBottomPlayers = len(allUsers)
		}

		bottomPlayers := allUsers[:numBottomPlayers]
		amountPerPlayer := poolAmount / float64(numBottomPlayers)

		var message string
		var affectedUsers []string

		for _, bottomPlayer := range bottomPlayers {
			var lockedPlayer models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedPlayer, bottomPlayer.ID).Error; err != nil {
				return err
			}

			lockedPlayer.Points += amountPerPlayer
			if err := tx.Save(&lockedPlayer).Error; err != nil {
				return err
			}

			playerName := lockedPlayer.Username
			displayName := ""
			if playerName == nil || *playerName == "" {
				displayName = fmt.Sprintf("<@%s>", lockedPlayer.DiscordID)
			} else {
				displayName = *playerName
			}
			affectedUsers = append(affectedUsers, displayName)
		}

		guild.Pool -= poolAmount
		if err := tx.Save(&guild).Error; err != nil {
			return err
		}

		if len(affectedUsers) == 1 {
			message = fmt.Sprintf("Black Hole activated! 25%% of the pool (%.0f points) was distributed to %s.", poolAmount, affectedUsers[0])
		} else {
			message = fmt.Sprintf("Black Hole activated! 25%% of the pool (%.0f points) was evenly distributed among the bottom %d players: %s.", poolAmount, numBottomPlayers, strings.Join(affectedUsers, ", "))
		}

		result = &models.CardResult{
			Message:     message,
			PointsDelta: 0,
			PoolDelta:   -poolAmount,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func handleSpyKids(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "View your super secret spy message to receive further instructions.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleDeath(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var allInventories []models.UserInventory
		if err := tx.Where("guild_id = ? AND deleted_at IS NULL", guildID).Find(&allInventories).Error; err != nil {
			return err
		}

		if len(allInventories) == 0 {
			result = &models.CardResult{
				Message:     "Death's transformation! No positive cards found in any player's inventory. The pool remains untouched.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		cardIDMap := make(map[uint]bool)
		for _, inventory := range allInventories {
			cardIDMap[inventory.CardID] = true
		}

		cardIDs := make([]uint, 0, len(cardIDMap))
		for cardID := range cardIDMap {
			cardIDs = append(cardIDs, cardID)
		}

		var cards []models.Card
		if err := tx.Where("id IN ?", cardIDs).Preload("CardRarity").Find(&cards).Error; err != nil {
			return err
		}

		cardMap := make(map[uint]*models.Card)
		for i := range cards {
			cardMap[cards[i].ID] = &cards[i]
		}

		var cardsToDestroy []models.UserInventory
		var destroyedCardNames []string
		cardNameCounts := make(map[string]int)

		for _, inventory := range allInventories {
			card, exists := cardMap[inventory.CardID]
			if !exists || card == nil {
				continue
			}

			if IsPositiveFromCode(inventory.CardID) && card.CardRarity.Name != "Mythic" {
				cardsToDestroy = append(cardsToDestroy, inventory)
				cardName := card.Name
				cardNameCounts[cardName]++
			}
		}

		if len(cardsToDestroy) == 0 {
			result = &models.CardResult{
				Message:     "Death's transformation! No positive non-Mythic cards found in any player's inventory. The pool remains untouched.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		for cardName, count := range cardNameCounts {
			if count == 1 {
				destroyedCardNames = append(destroyedCardNames, cardName)
			} else {
				destroyedCardNames = append(destroyedCardNames, fmt.Sprintf("%s (x%d)", cardName, count))
			}
		}

		for _, inventory := range cardsToDestroy {
			if err := tx.Delete(&inventory).Error; err != nil {
				return err
			}
		}

		cardsDestroyed := len(cardsToDestroy)
		poolDrain := float64(cardsDestroyed) * 100.0

		var guild models.Guild
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			First(&guild).Error; err != nil {
			return err
		}

		origPool := guild.Pool
		actualDrain := math.Min(poolDrain, origPool)
		guild.Pool -= actualDrain
		if guild.Pool < 0 {
			guild.Pool = 0
		}

		if err := tx.Save(&guild).Error; err != nil {
			return err
		}

		cardsList := strings.Join(destroyedCardNames, ", ")
		message := fmt.Sprintf("Death's transformation! Destroyed %d positive card(s): %s. %.0f points drained from the pool.", cardsDestroyed, cardsList, actualDrain)

		result = &models.CardResult{
			Message:     message,
			PointsDelta: 0,
			PoolDelta:   0,
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func handleTheTower(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	guild, err := guildService.GetGuildInfo(s, db, guildID, "")
	if err != nil {
		return nil, err
	}

	if err := db.Model(&models.User{}).
		Where("guild_id = ?", guildID).
		Update("points", gorm.Expr("GREATEST(0, points - ?)", 50)).Error; err != nil {
		return nil, err
	}

	poolReduction := guild.Pool * 0.75
	return &models.CardResult{
		Message:     "The Tower has fallen! The pool was reduced by 75% and every player lost 50 points as the debris settled.",
		PointsDelta: 0,
		PoolDelta:   -poolReduction,
	}, nil
}

func handleTheWorld(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, "")
		if err != nil {
			return err
		}

		poolWin := guild.Pool * 0.10

		var entries []models.BetEntry
		if err := tx.Preload("Bet").
			Joins("JOIN bets ON bets.id = bet_entries.bet_id").
			Where("bet_entries.user_id = ? AND bets.paid = ? AND bet_entries.deleted_at IS NULL", user.ID, false).
			Find(&entries).Error; err != nil {
			return err
		}

		if len(entries) == 0 {
			result = &models.CardResult{
				Message:     fmt.Sprintf("The World! You have no open bets to resolve, but you still receive 10%% of the pool (%.0f points).", poolWin),
				PointsDelta: poolWin,
				PoolDelta:   -poolWin,
			}
			return nil
		}

		randomIndex := rand.Intn(len(entries))
		entry := entries[randomIndex]
		bet := entry.Bet

		basePayout := common.CalculatePayout(entry.Amount, entry.Option, bet)
		totalWin := basePayout + poolWin

		if err := tx.Model(&user).UpdateColumn("total_bets_won", gorm.Expr("total_bets_won + 1")).Error; err != nil {
			return err
		}
		if err := tx.Model(&user).UpdateColumn("total_points_won", gorm.Expr("total_points_won + ?", totalWin)).Error; err != nil {
			return err
		}

		if err := tx.Delete(&entry).Error; err != nil {
			return err
		}

		betName := bet.Description
		result = &models.CardResult{
			Message:     fmt.Sprintf("The World! One of your open bets (**%s**) was resolved as a win. You received %.0f from the bet and 10%% of the pool (%.0f).", betName, basePayout, poolWin),
			PointsDelta: totalWin,
			PoolDelta:   -poolWin,
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func handleToTheMoon(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var result *models.CardResult
	if err := db.Transaction(func(tx *gorm.DB) error {
		var openBets []models.Bet
		if err := tx.Where("guild_id = ? AND paid = ? AND deleted_at IS NULL", guildID, false).Find(&openBets).Error; err != nil {
			return err
		}
		if len(openBets) == 0 {
			result = &models.CardResult{
				Message:     "To the Moon 🚀! There were no open bets to resolve.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		betIDs := make([]uint, len(openBets))
		for i, b := range openBets {
			betIDs[i] = b.ID
		}

		var entries []models.BetEntry
		if err := tx.Preload("Bet").Where("bet_id IN ? AND deleted_at IS NULL", betIDs).Find(&entries).Error; err != nil {
			return err
		}
		if len(entries) == 0 {
			result = &models.CardResult{
				Message:     "To the Moon 🚀! There were no bet entries on open bets to resolve.",
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		totalPayout := 0.0
		for _, entry := range entries {
			bet := entry.Bet
			payout := common.CalculatePayout(entry.Amount, entry.Option, bet)
			if err := tx.Model(&models.User{}).Where("id = ?", entry.UserID).Updates(map[string]interface{}{
				"points":           gorm.Expr("points + ?", payout),
				"total_bets_won":   gorm.Expr("total_bets_won + 1"),
				"total_points_won": gorm.Expr("total_points_won + ?", payout),
			}).Error; err != nil {
				return err
			}
			totalPayout += payout
			if err := tx.Delete(&entry).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&models.Bet{}).Where("id IN ?", betIDs).Updates(map[string]interface{}{
			"paid":   true,
			"active": false,
		}).Error; err != nil {
			return err
		}

		result = &models.CardResult{
			Message:     fmt.Sprintf("To the Moon 🚀! All %d open bet(s) were resolved as wins. %d entries paid out (%.0f points total).", len(openBets), len(entries), totalPayout),
			PointsDelta: 0,
			PoolDelta:   0,
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func handleHelmetSticker(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You've earned a helmet sticker! Gain 15 points.",
		PointsDelta: 15,
		PoolDelta:   0,
	}, nil
}

func handleWalkOn(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", userID, guildID).
		First(&user).Error; err != nil {
		return nil, err
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ? and deleted_at is null", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other players found. This card has no effect.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	userPosition := 0
	for i, u := range allUsers {
		if u.ID == user.ID {
			userPosition = i + 1
			break
		}
	}

	if userPosition == 0 {
		return &models.CardResult{
			Message:     "Could not determine your position. This card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	totalPlayers := len(allUsers)
	bottom50PercentThreshold := totalPlayers / 2
	if totalPlayers%2 != 0 {
		bottom50PercentThreshold = (totalPlayers + 1) / 2
	}

	isBottom50Percent := userPosition > bottom50PercentThreshold

	if isBottom50Percent {
		return &models.CardResult{
			Message:     "Congrats on making the team! +25 Points.",
			PointsDelta: 75,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:     "You did what was expected. Here's 10 points.",
		PointsDelta: 25,
		PoolDelta:   0,
	}, nil
}

func handleWaterboy(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", userID, guildID).
		First(&user).Error; err != nil {
		return nil, err
	}
	userMention := "<@" + userID + ">"

	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ? AND deleted_at IS NULL", guildID, userID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other players found to give points to. The card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	randomIndex := rand.Intn(len(allUsers))
	targetUser := allUsers[randomIndex]

	giveAmount := 10.0
	if user.Points < giveAmount {
		giveAmount = user.Points
	}

	poolReward := 20.0
	targetID := targetUser.DiscordID
	targetMention := "<@" + targetID + ">"
	return &models.CardResult{
		Message:           fmt.Sprintf("%s gave %.0f points to %s. The Pool rewards you with %.0f points for your service!", userMention, giveAmount, targetMention, poolReward),
		PointsDelta:       poolReward - giveAmount,
		PoolDelta:         -poolReward,
		TargetUserID:      &targetID,
		TargetPointsDelta: giveAmount,
	}, nil
}

func handleAlleyOop(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Alley-Oop requires you to select a target!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func handleTransferPortal(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Transfer Portal requires you to select a user who has a tradeable (non-Mythic) card to swap with.",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func handleBlindsideBlock(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Blindside Block requires you to select a target!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func ExecuteBlindsideBlock(db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var targetUser models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
		First(&targetUser).Error; err != nil {
		return nil, err
	}

	targetMention := "<@" + targetUserID + ">"

	// Check Moon protection first
	moonRedirected, err := CheckAndConsumeMoon(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if moonRedirected {
		// Get user who played the card for exclusion list
		var user models.User
		if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
			return nil, err
		}

		randomUserID, err := GetRandomUserForMoon(db, guildID, []uint{targetUser.ID, user.ID})
		if err != nil {
			// No eligible users found, check Shield/Redshirt
			blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, targetUser.ID, guildID)
			if err != nil {
				return nil, err
			}
			if blocked {
				targetID := targetUserID
				return &models.CardResult{
					Message:           fmt.Sprintf("Blindside block! %s's Moon illusion tried to redirect, but no eligible users found. %s blocked it instead!", targetMention, protectionName(blockedByRedshirt)),
					PointsDelta:       0,
					PoolDelta:         0,
					TargetUserID:      &targetID,
					TargetPointsDelta: 0,
				}, nil
			}
			targetID := targetUserID
			return &models.CardResult{
				Message:           fmt.Sprintf("Blindside block! %s's Moon illusion tried to redirect, but no eligible users found. The card fizzles out.", targetMention),
				PointsDelta:       0,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: 0,
			}, nil
		}

		// Moon redirected successfully - apply to random user
		var randomUser models.User
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", randomUserID, guildID).
			First(&randomUser).Error; err != nil {
			return nil, err
		}

		deductAmount := 50.0
		if randomUser.Points < deductAmount {
			deductAmount = randomUser.Points
		}

		randomUser.Points -= deductAmount
		if err := db.Save(&randomUser).Error; err != nil {
			return nil, err
		}

		var guild models.Guild
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			First(&guild).Error; err != nil {
			return nil, err
		}
		guild.Pool += deductAmount
		if err := db.Save(&guild).Error; err != nil {
			return nil, err
		}

		randomMention := "<@" + randomUserID + ">"
		targetID := randomUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("Blindside block! %s's Moon illusion redirected it! %s lost %.0f points (added to the Pool).", targetMention, randomMention, deductAmount),
			PointsDelta:       0,
			PoolDelta:         deductAmount,
			TargetUserID:      &targetID,
			TargetPointsDelta: -deductAmount,
		}, nil
	}

	// Moon didn't redirect, check Shield/Redshirt
	blocked, blockedByRedshirt, err := CheckAndConsumeShieldOrRedshirt(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if blocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("Blindside block! %s's %s blocked it!", targetMention, protectionName(blockedByRedshirt)),
			PointsDelta:       0,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: 0,
		}, nil
	}

	// No protection - proceed with normal deduction
	deductAmount := 50.0
	if targetUser.Points < deductAmount {
		deductAmount = targetUser.Points
	}

	targetUser.Points -= deductAmount
	if err := db.Save(&targetUser).Error; err != nil {
		return nil, err
	}

	var guild models.Guild
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("guild_id = ?", guildID).
		First(&guild).Error; err != nil {
		return nil, err
	}
	guild.Pool += deductAmount
	if err := db.Save(&guild).Error; err != nil {
		return nil, err
	}

	targetID := targetUserID
	return &models.CardResult{
		Message:           fmt.Sprintf("Blindside block! Your target lost %.0f points (added to the Pool).", deductAmount),
		PointsDelta:       0,
		PoolDelta:         deductAmount,
		TargetUserID:      &targetID,
		TargetPointsDelta: -deductAmount,
	}, nil
}

func ExecuteAlleyOop(db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", userID, guildID).
		First(&user).Error; err != nil {
		return nil, err
	}

	var targetUser models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
		First(&targetUser).Error; err != nil {
		return nil, err
	}

	giveAmount := 50.0
	if user.Points < giveAmount {
		giveAmount = user.Points
	}

	user.Points -= giveAmount
	targetUser.Points += giveAmount

	if err := db.Save(&user).Error; err != nil {
		return nil, err
	}
	if err := db.Save(&targetUser).Error; err != nil {
		return nil, err
	}

	targetID := targetUserID
	return &models.CardResult{
		Message:           fmt.Sprintf("Perfect assist! You gave %.0f points to your teammate.", giveAmount),
		PointsDelta:       -giveAmount,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: giveAmount,
	}, nil
}

func handleAlumniDonation(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "A wealthy booster paid your NIL. Gain 50 points from the Pool.",
		PointsDelta: 50,
		PoolDelta:   -50,
	}, nil
}

func handleAwayGame(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "Travel costs are high. Lose 10 points.",
		PointsDelta: -10,
		PoolDelta:   10,
	}, nil
}

func handleSchoolSpirit(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", userID, guildID).
		First(&user).Error; err != nil {
		return nil, err
	}
	userMention := "<@" + userID + ">"

	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ? AND deleted_at IS NULL", guildID, userID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other players found. This card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	randomIndex := rand.Intn(len(allUsers))
	targetUser := allUsers[randomIndex]

	giveAmount := 10.0
	if user.Points < giveAmount {
		giveAmount = user.Points
	}

	targetID := targetUser.DiscordID
	targetMention := "<@" + targetID + ">"
	return &models.CardResult{
		Message:           fmt.Sprintf("%s and %s both gained 10 points.", userMention, targetMention),
		PointsDelta:       10,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: giveAmount,
	}, nil
}

func handleSixthMan(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", userID, guildID).
		First(&user).Error; err != nil {
		return nil, err
	}
	userMention := "<@" + userID + ">"

	var allUsers []models.User
	if err := db.Where("guild_id = ? AND deleted_at IS NULL", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other players found. This card has no effect.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	userPosition := 0
	for i, u := range allUsers {
		if u.ID == user.ID {
			userPosition = i + 1
			break
		}
	}

	if userPosition == 0 {
		return &models.CardResult{
			Message:     "Could not determine your position. This card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	if userPosition == 11 {
		return &models.CardResult{
			Message:     fmt.Sprintf("%s enters the game! You're the first player outside the top 10. Gain 150 points!", userMention),
			PointsDelta: 150,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:     "You're already in the top 10. Here's 50 points.",
		PointsDelta: 50,
		PoolDelta:   0,
	}, nil
}

func handleStormTheField(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ? AND deleted_at IS NULL", guildID, userID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other players found. This card has no effect.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	for _, user := range allUsers {
		user.Points += 150
		if err := db.Save(&user).Error; err != nil {
			return nil, err
		}
	}

	return &models.CardResult{
		Message:     "Storm the Field! Everyone in the server gains 150 points, but the Pool is reduced by 1,500 points in security fines.",
		PointsDelta: 150,
		PoolDelta:   -1500,
	}, nil
}

func handleRecruitingViolation(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", userID, guildID).
		First(&user).Error; err != nil {
		return nil, err
	}

	deductAmount := user.Points * 0.05
	if user.Points < deductAmount {
		deductAmount = user.Points
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("Recruiting Violation! You lost %.0f points to the Pool.", deductAmount),
		PointsDelta: -(deductAmount),
		PoolDelta:   deductAmount,
	}, nil
}

func handleNcaaSanctions(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var guild models.Guild
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("guild_id = ?", guildID).
		First(&guild).Error; err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     "NCAA Sanctions! The Pool is reduced by 15%.",
		PointsDelta: 0,
		PoolDelta:   -guild.Pool * 0.15,
	}, nil
}

func handleMarchMadness(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ? AND deleted_at IS NULL", guildID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "March Madness! No active players in the server. Chaos fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	messageDetails := ""
	shown := 0
	const maxLines = 25

	for i := range allUsers {
		// Random multiplier in [-0.05, +0.05] (i.e. -5% to +5%)
		multiplier := rand.Float64()*0.1 - 0.05
		pointsChange := allUsers[i].Points * multiplier

		allUsers[i].Points += pointsChange
		if allUsers[i].Points < 0 {
			allUsers[i].Points = 0
		}

		if err := db.Save(&allUsers[i]).Error; err != nil {
			return nil, err
		}

		if shown < maxLines {
			username := allUsers[i].Username
			displayName := ""
			if username == nil || *username == "" {
				displayName = fmt.Sprintf("<@%s>", allUsers[i].DiscordID)
			} else {
				displayName = *username
			}
			sign := "+"
			if pointsChange < 0 {
				sign = ""
			}
			messageDetails += fmt.Sprintf("%s: %s%.0f points\n", displayName, sign, pointsChange)
			shown++
		}
	}

	if len(allUsers) > maxLines {
		messageDetails += fmt.Sprintf("\n...and %d more.", len(allUsers)-maxLines)
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("March Madness! Every player's balance was modified by a random amount between -5%% and +5%%:\n\n%s", messageDetails),
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleDarkHorse(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ? AND deleted_at IS NULL", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found. Dark Horse fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	userPosition := 0
	for i, u := range allUsers {
		if u.ID == user.ID {
			userPosition = i + 1
			break
		}
	}

	if userPosition == 0 {
		return &models.CardResult{
			Message:     "Could not determine your position. Dark Horse fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	totalPlayers := len(allUsers)
	bottom25PercentThreshold := totalPlayers - (totalPlayers / 4)
	if totalPlayers%4 != 0 {
		bottom25PercentThreshold = totalPlayers - ((totalPlayers + 3) / 4)
	}
	isBottom25Percent := userPosition > bottom25PercentThreshold

	if isBottom25Percent {
		return &models.CardResult{
			Message:     "Dark Horse! You're in the bottom 25% of players. You gain 500 points!",
			PointsDelta: 500,
			PoolDelta:   0,
		}, nil
	}
	return &models.CardResult{
		Message:     "Dark Horse! You're not in the bottom 25%. You gain 50 points.",
		PointsDelta: 50,
		PoolDelta:   0,
	}, nil
}

func handleBracketBuster(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Bracket Buster requires you to select a user! Their smallest active bet will be cancelled and the wager sent to the pool.",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

func handleHeismanTrophy(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	guild, err := guildService.GetGuildInfo(s, db, guildID, "")
	if err != nil {
		return nil, err
	}
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	poolWin := guild.Pool * 0.05

	inv := models.UserInventory{
		UserID:    user.ID,
		GuildID:   guildID,
		CardID:    DoubleDownCardID,
		CardCode:  "DDN",
		ExpiresAt: nil,
	}
	if err := db.Create(&inv).Error; err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     "You won the Heisman Trophy (🏟️)! You won 5% of the pool and received a Double Down card. Your next winning bet payout is doubled!",
		PointsDelta: poolWin,
		PoolDelta:   -poolWin,
	}, nil
}

func handleNationalChampionship(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var guild models.Guild
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("guild_id = ?", guildID).
		First(&guild).Error; err != nil {
		return nil, err
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ? AND deleted_at IS NULL", guildID, userID).Find(&allUsers).Error; err != nil {
		return nil, err
	}
	poolWin := guild.Pool * 0.2
	var otherUserPointsDelta float64
	if len(allUsers) == 0 {
		otherUserPointsDelta = 0
	} else {
		otherUserPointsDelta = math.Min(200.0, (guild.Pool-poolWin)/float64(len(allUsers)))
	}
	for _, user := range allUsers {
		user.Points += otherUserPointsDelta
		if err := db.Save(&user).Error; err != nil {
			return nil, err
		}
	}

	totalDrain := poolWin + otherUserPointsDelta*float64(len(allUsers))
	guild.Pool -= totalDrain
	if err := db.Save(&guild).Error; err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("National Championship! You win 20%% of the current Pool, and every other active player gains %.0f points.", otherUserPointsDelta),
		PointsDelta: poolWin,
		PoolDelta:   -totalDrain,
	}, nil
}
