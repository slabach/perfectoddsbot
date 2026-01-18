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

	poolWin := guild.Pool * 0.5

	return &models.CardResult{
		Message:     "You discovered the Holy Grail! You won 50% of the pool!",
		PointsDelta: poolWin,
		PoolDelta:   -poolWin,
	}, nil
}

func handleJackpot(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	guild, err := guildService.GetGuildInfo(s, db, guildID, "")
	if err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     ":rotating_light: You discovered the JACKPOT! You won 100% of the pool! :rotating_light:",
		PointsDelta: guild.Pool,
		PoolDelta:   -guild.Pool,
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
	// Get both users
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var targetUser models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).First(&targetUser).Error; err != nil {
		return nil, err
	}

	// Check for Shield
	blocked, err := checkAndConsumeShield(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if blocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           "Their Shield blocked the theft attempt!",
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

	targetID := targetUserID
	return &models.CardResult{
		Message:           "You successfully pickpocketed your target!",
		PointsDelta:       stealAmount,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: -stealAmount,
	}, nil
}

func handleNilFee(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	// Don't deduct more than user has
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

// handleSmallRebate refunds the cost of buying this card
func handleSmallRebate(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user to determine card cost
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
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var userAbove models.User
	result := db.Where("guild_id = ? AND (points > ? OR (points = ? AND id < ?))", guildID, user.Points, user.Points, user.ID).
		Order("points DESC, id ASC").
		First(&userAbove)

	if result.Error != nil || userAbove.ID == 0 {
		return &models.CardResult{
			Message:     "You're at the top of the leaderboard! No one to tip you. The card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	transferAmount := 10.0
	if userAbove.Points < transferAmount {
		transferAmount = userAbove.Points
	}

	targetID := userAbove.DiscordID
	return &models.CardResult{
		Message:           fmt.Sprintf("You shook the tip jar! The person above you gave you %.1f points.", transferAmount),
		PointsDelta:       transferAmount,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: -transferAmount,
	}, nil
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

	timeoutUntil := time.Now().Add(2 * time.Hour)
	user.CardDrawTimeoutUntil = &timeoutUntil

	if err := db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     "You've been timed out! You cannot buy another card for 1 hour.",
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

	// Get all users in guild ordered by points (descending)
	var allUsers []models.User
	if err := db.Where("guild_id = ?", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other players found. The charity has no one to help.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	// Find user's position (1-indexed from top)
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
	if err := db.Where("guild_id = ?", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
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

func handlePickpocketCommon(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ?", guildID, userID).Find(&allUsers).Error; err != nil {
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
	if err := db.Where("guild_id = ? AND discord_id != ?", guildID, userID).Find(&allUsers).Error; err != nil {
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

	// Pick a random user
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

// handlePocketSand refunds the cost of drawing the card
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

func handleMajorGlitch(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ?", guildID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No users found in the server. The glitch fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	gainAmount := 100.0
	updatedCount := 0
	for i := range allUsers {
		if allUsers[i].DiscordID != userID {
			allUsers[i].Points += gainAmount
			if err := db.Save(&allUsers[i]).Error; err != nil {
				return nil, err
			}
			updatedCount++
		}
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("A major glitch occurred! Everyone in the server gained %.0f points! (%d users affected)", gainAmount, updatedCount+1),
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
	if err := db.Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).First(&targetUser).Error; err == nil {
		blocked, err := checkAndConsumeShield(db, targetUser.ID, guildID)
		if err != nil {
			return nil, err
		}
		if blocked {
			return &models.CardResult{
				Message:           fmt.Sprintf("@%s's Shield blocked the Jester's curse!", targetMention),
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

func removeCardFromInventory(db *gorm.DB, userID uint, guildID string, cardID int) error {
	var item models.UserInventory
	result := db.Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cardID).First(&item)
	if result.Error != nil {
		return result.Error
	}
	return db.Delete(&item).Error
}

func handleStimulusCheck(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ?", guildID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No users found in the server. The stimulus check bounces.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	// Update all users except the current user (current user will get points via PointsDelta)
	gainAmount := 50.0
	updatedCount := 0
	for i := range allUsers {
		if allUsers[i].DiscordID != userID {
			allUsers[i].Points += gainAmount
			if err := db.Save(&allUsers[i]).Error; err != nil {
				return nil, err
			}
			updatedCount++
		}
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("Stimulus Check arrived! Everyone in the server gained %.0f points! (%d users affected)", gainAmount, updatedCount+1),
		PointsDelta: gainAmount, // Current user gets points through normal flow
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
	// Get user and guild to determine card cost
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
	if err := db.Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).First(&targetUser).Error; err != nil {
		return nil, err
	}

	blocked, err := checkAndConsumeShield(db, targetUser.ID, guildID)
	if err != nil {
		return nil, err
	}
	if blocked {
		targetID := targetUserID
		return &models.CardResult{
			Message:           "Their Shield blocked the Bet Freeze!",
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
		Message:           "Target's betting ability has been frozen for 2 hours!",
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
	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ?", guildID, userID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No other users found to target with Green Shells. The shells break against the wall.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
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
		blocked, err := checkAndConsumeShield(db, target.ID, guildID)
		if err != nil {
			return nil, err
		}

		targetName := target.Username
		displayName := ""
		if targetName == nil || *targetName == "" {
			displayName = fmt.Sprintf("<@%s>", target.DiscordID)
		} else {
			displayName = *targetName
		}

		if blocked {
			message += fmt.Sprintf("%s's Shield blocked a shell! ", displayName)
			continue
		}

		loss := float64(rand.Intn(25) + 1)

		if target.Points < loss {
			loss = target.Points
		}

		target.Points -= loss
		if err := db.Save(&target).Error; err != nil {
			return nil, err
		}

		message += fmt.Sprintf("%s was hit for %.0f points! ", displayName, loss)
	}

	if message == "" {
		message = "Green Shells were thrown but missed everyone (or were all blocked)!"
	} else {
		message = "Green Shells thrown! " + message
	}

	return &models.CardResult{
		Message:     message,
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleWhackAMole(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ?", guildID, userID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No moles found to whack! The hammer hits the empty ground.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
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
		blocked, err := checkAndConsumeShield(db, target.ID, guildID)
		if err != nil {
			return nil, err
		}

		targetName := target.Username
		displayName := ""
		if targetName == nil || *targetName == "" {
			displayName = fmt.Sprintf("<@%s>", target.DiscordID)
		} else {
			displayName = *targetName
		}

		if blocked {
			message += fmt.Sprintf("%s blocked the hammer! ", displayName)
			continue
		}

		loss := float64(rand.Intn(10) + 1)

		if target.Points < loss {
			loss = target.Points
		}

		target.Points -= loss
		if err := db.Save(&target).Error; err != nil {
			return nil, err
		}

		message += fmt.Sprintf("%s whacked for %.0f! ", displayName, loss)
	}

	if message == "" {
		message = "You swung the hammer but hit nothing (or all blocked)!"
	} else {
		message = "Whack-a-Mole! " + message
	}

	return &models.CardResult{
		Message:     message,
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
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
	var allUsers []models.User
	if err := db.Where("guild_id = ?", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found in the server. Socialism has no one to redistribute from.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
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
		return &models.CardResult{
			Message:     "No top players found to take points from. Socialism fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	if len(bottomPlayers) == 0 {
		return &models.CardResult{
			Message:     "No bottom players found to give points to. Socialism fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	var totalCollected float64
	var topMessage string
	var bottomMessage string

	for _, topPlayer := range topPlayers {
		blocked, err := checkAndConsumeShield(db, topPlayer.ID, guildID)
		if err != nil {
			return nil, err
		}

		topPlayerName := topPlayer.Username
		topDisplayName := ""
		if topPlayerName == nil || *topPlayerName == "" {
			topDisplayName = fmt.Sprintf("<@%s>", topPlayer.DiscordID)
		} else {
			topDisplayName = *topPlayerName
		}

		if blocked {
			topMessage += fmt.Sprintf("%s's Shield blocked!\n", topDisplayName)
			continue
		}

		takeAmount := 90.0
		if topPlayer.Points < takeAmount {
			takeAmount = topPlayer.Points
		}

		if takeAmount > 0 {
			topPlayer.Points -= takeAmount
			totalCollected += takeAmount
			if err := db.Save(&topPlayer).Error; err != nil {
				return nil, err
			}
			topMessage += fmt.Sprintf("%s lost %.0f points\n", topDisplayName, takeAmount)
		}
	}

	if totalCollected > 0 && len(bottomPlayers) > 0 {
		amountPerBottomPlayer := totalCollected / float64(len(bottomPlayers))

		for _, bottomPlayer := range bottomPlayers {
			bottomPlayer.Points += amountPerBottomPlayer
			if err := db.Save(&bottomPlayer).Error; err != nil {
				return nil, err
			}

			bottomPlayerName := bottomPlayer.Username
			bottomDisplayName := ""
			if bottomPlayerName == nil || *bottomPlayerName == "" {
				bottomDisplayName = fmt.Sprintf("<@%s>", bottomPlayer.DiscordID)
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

	return &models.CardResult{
		Message:     message,
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleRobinHood(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ?", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found in the server. Robin Hood has no one to rob from.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	if len(allUsers) == 1 {
		return &models.CardResult{
			Message:     "Only one player in the server. Robin Hood fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	topPlayer := allUsers[0]
	bottomPlayer := allUsers[len(allUsers)-1]

	if topPlayer.ID == bottomPlayer.ID {
		return &models.CardResult{
			Message:     "The richest and poorest players are the same! Robin Hood has no one to redistribute from.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	topPlayerName := topPlayer.Username
	topDisplayName := ""
	if topPlayerName == nil || *topPlayerName == "" {
		topDisplayName = fmt.Sprintf("<@%s>", topPlayer.DiscordID)
	} else {
		topDisplayName = *topPlayerName
	}

	bottomPlayerName := bottomPlayer.Username
	bottomDisplayName := ""
	if bottomPlayerName == nil || *bottomPlayerName == "" {
		bottomDisplayName = fmt.Sprintf("<@%s>", bottomPlayer.DiscordID)
	} else {
		bottomDisplayName = *bottomPlayerName
	}

	blocked, err := checkAndConsumeShield(db, topPlayer.ID, guildID)
	if err != nil {
		return nil, err
	}

	if blocked {
		return &models.CardResult{
			Message:     fmt.Sprintf("Robin Hood attempted to steal from %s, but their Shield parried the theif! The card fizzles out.", topDisplayName),
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	takeAmount := 200.0
	if topPlayer.Points < takeAmount {
		takeAmount = topPlayer.Points
	}

	if takeAmount <= 0 {
		return &models.CardResult{
			Message:     fmt.Sprintf("Robin Hood attempted to steal from %s, but they have no points! The card fizzles out.", topDisplayName),
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	topPlayer.Points -= takeAmount
	if err := db.Save(&topPlayer).Error; err != nil {
		return nil, err
	}

	bottomPlayer.Points += 150.0
	if err := db.Save(&bottomPlayer).Error; err != nil {
		return nil, err
	}

	message := fmt.Sprintf("Robin Hood strikes! Stole %.0f points from %s, gave 150 to %s, and kept 50 for yourself!", takeAmount, topDisplayName, bottomDisplayName)

	return &models.CardResult{
		Message:     message,
		PointsDelta: 50.0,
		PoolDelta:   0,
	}, nil
}

func handleRedShells(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ?", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found in the server. Red Shells have no targets.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	var drawerIndex int = -1
	for i, user := range allUsers {
		if user.DiscordID == userID {
			drawerIndex = i
			break
		}
	}

	if drawerIndex == -1 {
		return &models.CardResult{
			Message:     "Could not find your position on the leaderboard. Red Shells break against the wall.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	if drawerIndex == 0 {
		return &models.CardResult{
			Message:     "You're at the top of the leaderboard! There's no one in front of you to hit with Red Shells. Red Shells break against the wall.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
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
		return &models.CardResult{
			Message:     "No targets found in front of you. Red Shells break against the wall.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	var message string

	for _, target := range targets {
		blocked, err := checkAndConsumeShield(db, target.ID, guildID)
		if err != nil {
			return nil, err
		}

		targetName := target.Username
		displayName := ""
		if targetName == nil || *targetName == "" {
			displayName = fmt.Sprintf("<@%s>", target.DiscordID)
		} else {
			displayName = *targetName
		}

		if blocked {
			message += fmt.Sprintf("%s's Shield blocked a shell! ", displayName)
			continue
		}

		loss := float64(rand.Intn(26) + 25)

		if target.Points < loss {
			loss = target.Points
		}

		if loss > 0 {
			target.Points -= loss
			if err := db.Save(&target).Error; err != nil {
				return nil, err
			}
			message += fmt.Sprintf("%s was hit for %.0f points! ", displayName, loss)
		}
	}

	if message == "" {
		message = "Red Shells were thrown but all were blocked!"
	} else {
		message = "Red Shells thrown! " + message
	}

	return &models.CardResult{
		Message:     message,
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
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

func handleGrandLarceny(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Grand Larceny requires you to select a target!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
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
	var allUsers []models.User
	if err := db.Where("guild_id = ?", guildID).Order("points DESC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found in the server. The Blue Shell breaks against the wall.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	firstPlaceUser := allUsers[0]

	if firstPlaceUser.DiscordID == userID {
		return &models.CardResult{
			Message:     "You're in 1st place! The Blue Shell targets you, but you're already at the top. Blue Shell breaks against the wall.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	blocked, err := checkAndConsumeShield(db, firstPlaceUser.ID, guildID)
	if err != nil {
		return nil, err
	}

	if blocked {
		firstPlaceUsername := firstPlaceUser.Username
		displayName := ""
		if firstPlaceUsername == nil || *firstPlaceUsername == "" {
			displayName = fmt.Sprintf("<@%s>", firstPlaceUser.DiscordID)
		} else {
			displayName = *firstPlaceUsername
		}

		return &models.CardResult{
			Message:     fmt.Sprintf("The Blue Shell was thrown at %s, but their Shield blocked it!", displayName),
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
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
		if err := db.Save(&firstPlaceUser).Error; err != nil {
			return nil, err
		}
	}

	firstPlaceUsername := firstPlaceUser.Username
	displayName := ""
	if firstPlaceUsername == nil || *firstPlaceUsername == "" {
		displayName = fmt.Sprintf("<@%s>", firstPlaceUser.DiscordID)
	} else {
		displayName = *firstPlaceUsername
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("The Blue Shell hit %s! They lost %.0f points to the Pool.", displayName, deductAmount),
		PointsDelta: 0,
		PoolDelta:   deductAmount,
	}, nil
}

func handleNuke(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ?", guildID).Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found in the server. The Nuke fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	for _, user := range allUsers {
		user.Points -= user.Points * 0.1
		if err := db.Save(&user).Error; err != nil {
			return nil, err
		}
	}

	return &models.CardResult{
		Message:     "You've drawn The Nuke! Everyone (including you) loses 10% of their points to the Pool.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleDivineIntervention(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var allUsers []models.User
	if err := db.Where("guild_id = ?", guildID).Find(&allUsers).Error; err != nil {
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

	user.Points = averagePoints
	if err := db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("Your points balance is set to exactly the average of all players (%.2f).", averagePoints),
		PointsDelta: 0,
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
	var allUsers []models.User
	if err := db.Where("guild_id = ?", guildID).Order("points ASC").Find(&allUsers).Error; err != nil {
		return nil, err
	}

	if len(allUsers) == 0 {
		return &models.CardResult{
			Message:     "No players found in the server. Robbing the Hood fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	poorestUser := allUsers[0]

	if poorestUser.DiscordID == userID {
		return &models.CardResult{
			Message:     "You're the poorest player! Robbing the Hood fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	return &models.CardResult{
		Message:           "You've drawn Robbing the Hood! You stole 50% of the poorest player's points and gave it to yourself.",
		PointsDelta:       poorestUser.Points * 0.5,
		PoolDelta:         0,
		TargetUserID:      &poorestUser.DiscordID,
		TargetPointsDelta: -(poorestUser.Points * 0.5),
	}, nil
}

func handleStopTheSteal(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// This card is UserPlayable and should be played manually via /play-card
	// If drawn normally, add it to inventory
	return &models.CardResult{
		Message:     "You drew STOP THE STEAL! Use /play-card to play this card and cancel any of your active bets.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleSnipSnap(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	var entries []models.BetEntry
	if err := db.Preload("Bet").
		Joins("JOIN bets ON bets.id = bet_entries.bet_id").
		Where("bet_entries.user_id = ? AND bets.paid = ? AND bet_entries.deleted_at IS NULL", user.ID, false).
		Find(&entries).Error; err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return &models.CardResult{
			Message:     "You have no active bets to flip. The card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	// Pick random entry
	randomIndex := rand.Intn(len(entries))
	entryToFlip := entries[randomIndex]

	// Flip option
	oldOption := entryToFlip.Option
	newOption := 0
	if oldOption == 1 {
		newOption = 2
	} else {
		newOption = 1
	}

	entryToFlip.Option = newOption
	if err := db.Save(&entryToFlip).Error; err != nil {
		return nil, err
	}

	betName := entryToFlip.Bet.Description
	newOptionName := ""
	if newOption == 1 {
		newOptionName = entryToFlip.Bet.Option1
	} else {
		newOptionName = entryToFlip.Bet.Option2
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("Snip Snap Snip Snap! Your bet on **%s** has been flipped! You are now betting on **%s**.", betName, newOptionName),
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

func handleGambler(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You drew The Gambler! You must choose your fate...",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}
