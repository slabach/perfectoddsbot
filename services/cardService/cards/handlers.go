package cards

import (
	"fmt"
	"math/rand"
	"perfectOddsBot/models"
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

	var targetUser models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
		First(&targetUser).Error; err != nil {
		return nil, err
	}

	targetMention := "<@" + targetUserID + ">"

	blocked, err := checkAndConsumeShield(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if blocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's Shield blocked the theft attempt!", targetMention),
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
			bountyMessage = fmt.Sprintf(" You also claimed 1 bounty (+%.0f points from pool)!", bountyReward)
		} else {
			bountyMessage = fmt.Sprintf(" You also claimed %d bounties (+%.0f points from pool)!", bountyCount, bountyReward)
		}
	}

	return &models.CardResult{
		Message:           fmt.Sprintf("You successfully pickpocketed your target!%s", bountyMessage),
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
		refundAmount = guild.CardDrawCost * 100
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

	shieldBlocked, err := checkAndConsumeShield(db, user.ID, guildID)
	if err != nil {
		return nil, err
	}
	if shieldBlocked {
		return &models.CardResult{
			Message:     fmt.Sprintf("<@%s>'s Shield blocked the Timeout! The card fizzles out.", userID),
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

func handleCoupon(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You found a coupon! Your next card purchase will be 25% off.",
		PointsDelta: 0,
		PoolDelta:   0,
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
		refundAmount = guild.CardDrawCost * 100
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
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
		First(&targetUser).Error; err == nil {
		blocked, err := checkAndConsumeShield(db, targetUser.ID, guildID)
		if err != nil {
			return nil, err
		}
		if blocked {
			return &models.CardResult{
				Message:           fmt.Sprintf("%s's Shield blocked the Jester's curse!", targetMention),
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
				Message:           fmt.Sprintf("%s's Spare Key blocked the Jester's curse!", targetMention),
				PointsDelta:       0,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: 0,
			}, nil
		}
	}

	timeoutUntil := time.Now().Add(15 * time.Minute)

	err := s.GuildMemberTimeout(guildID, targetUserID, &timeoutUntil)
	if err != nil {
		return &models.CardResult{
			Message:     "Failed to mute the target! They might be an Admin or too powerful.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:           fmt.Sprintf("The Jester laughs! %s has been muted for 15 minutes.", targetMention),
		PointsDelta:       0,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: 0,
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

	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, guildID, ShieldCardID).
		Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count > 0 {
		if err := removeCardFromInventory(db, user.ID, guildID, GenerousDonationCardID); err != nil {
			return nil, fmt.Errorf("failed to remove blocked donation card: %v", err)
		}
		if err := removeCardFromInventory(db, user.ID, guildID, ShieldCardID); err != nil {
			return nil, fmt.Errorf("failed to consume shield: %v", err)
		}

		return &models.CardResult{
			Message:     "Your Shield blocked the Generous Donation! The card fizzles out.",
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
		cardCost = guild.CardDrawCost * 100
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
	var targetUser models.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
		First(&targetUser).Error; err != nil {
		return nil, err
	}
	targetMention := "<@" + targetUserID + ">"

	blocked, err := checkAndConsumeShield(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if blocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's Shield blocked the Bet Freeze!", targetMention),
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
		Message:           fmt.Sprintf("%s's betting ability has been frozen for 2 hours!", targetMention),
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

			blocked, err := checkAndConsumeShield(tx, lockedTarget.ID, guildID)
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
				message += fmt.Sprintf("%s's Shield blocked a shell! ", displayName)
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

			blocked, err := checkAndConsumeShield(tx, lockedTarget.ID, guildID)
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
				message += fmt.Sprintf("%s blocked the hammer! ", displayName)
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

			blocked, err := checkAndConsumeShield(tx, lockedTop.ID, guildID)
			if err != nil {
				return err
			}

			topPlayerName := lockedTop.Username
			topDisplayName := ""
			if topPlayerName == nil || *topPlayerName == "" {
				topDisplayName = fmt.Sprintf("<@%s>", lockedTop.DiscordID)
			} else {
				topDisplayName = *topPlayerName
			}

			if blocked {
				topMessage += fmt.Sprintf("%s's Shield blocked!\n", topDisplayName)
				continue
			}

			takeAmount := 90.0
			if lockedTop.Points < takeAmount {
				takeAmount = lockedTop.Points
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

		blocked, err := checkAndConsumeShield(tx, lockedTop.ID, guildID)
		if err != nil {
			return err
		}

		if blocked {
			result = &models.CardResult{
				Message:     fmt.Sprintf("Robin Hood attempted to steal from %s, but their Shield parried the thief! The card fizzles out.", topDisplayName),
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

			blocked, err := checkAndConsumeShield(tx, lockedTarget.ID, guildID)
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
				message += fmt.Sprintf("%s's Shield blocked a shell! ", displayName)
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

func checkAndConsumeShield(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, ShieldCardID).
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

func checkAndConsumeSpareKey(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, SpareKeyCardID).
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
	return &models.CardResult{
		Message:           fmt.Sprintf("The Gossip spreads! %s's current point balance is **%.1f points**.", targetMention, targetUser.Points),
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
		Message:     "You've drawn The Vampire! For the next 24 hours, you'll earn 1% of every bet won by other players.",
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

		blocked, err := checkAndConsumeShield(tx, firstPlaceUser.ID, guildID)
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
				Message:     fmt.Sprintf("The Blue Shell was thrown at %s, but their Shield blocked it!", displayName),
				PointsDelta: 0,
				PoolDelta:   0,
			}
			return nil
		}

		deductAmount := 500.0
		if firstPlaceUser.Points < deductAmount {
			deductAmount = firstPlaceUser.Points
		}

		if deductAmount > 0 {
			firstPlaceUser.Points -= deductAmount
			if firstPlaceUser.Points < 0 {
				firstPlaceUser.Points = 0
			}
			if err := tx.Save(&firstPlaceUser).Error; err != nil {
				return err
			}
		}

		firstPlaceUsername := firstPlaceUser.Username
		displayName := ""
		if firstPlaceUsername == nil || *firstPlaceUsername == "" {
			displayName = fmt.Sprintf("<@%s>", firstPlaceUser.DiscordID)
		} else {
			displayName = *firstPlaceUsername
		}

		result = &models.CardResult{
			Message:     fmt.Sprintf("The Blue Shell hit %s! They lost %.0f points to the Pool.", displayName, deductAmount),
			PointsDelta: 0,
			PoolDelta:   deductAmount,
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

		stolenAmount := poorestUser.Points * 0.1
		poorestUser.Points -= stolenAmount
		if err := tx.Save(&poorestUser).Error; err != nil {
			return err
		}

		var drawer models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&drawer).Error; err != nil {
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

		user.Points -= userLoss
		lockedTarget.Points -= targetLoss

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
			Message:           fmt.Sprintf("🥔 Hot Potato! %s lost %.0f points. The hot potato was passed to %s who also lost %.0f points!", userMention, userLoss, targetMention, targetLoss),
			PointsDelta:       -userLoss,
			PoolDelta:         0,
			TargetUserID:      &targetID,
			TargetPointsDelta: -targetLoss,
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

			user.Points -= userLoss
			targetUser.Points -= targetLoss

			if err := tx.Save(&user).Error; err != nil {
				return err
			}
			if err := tx.Save(&targetUser).Error; err != nil {
				return err
			}

			result = &models.CardResult{
				Message:           fmt.Sprintf("⚔️ DUEL! You both rolled %d! It's a tie! You both lose 50 points.", userRoll),
				PointsDelta:       -userLoss,
				PoolDelta:         0,
				TargetUserID:      &targetID,
				TargetPointsDelta: -targetLoss,
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
			// Keep the oldest one (first in list), delete the rest
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

	blocked, err := checkAndConsumeShield(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if blocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           fmt.Sprintf("%s's Shield blocked the Social Distancing!", targetMention),
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
