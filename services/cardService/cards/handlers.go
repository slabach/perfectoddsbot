package cards

import (
	"fmt"
	"math/rand"
	"perfectOddsBot/models"
	"perfectOddsBot/services/guildService"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

// handleDud is a simple card that does nothing
func handleDud(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "Nothing happened. You drew a blank card.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

// handlePenny gives the user 1 point
func handlePenny(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You found a penny! +1 Point.",
		PointsDelta: 1,
		PoolDelta:   0,
	}, nil
}

// handlePapercut makes the user lose 10 points
func handlePapercut(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user to check current points (after card cost was deducted)
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	// Don't deduct more than user has
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

// handleVendingMachine gives the user 25 points
func handleVendingMachine(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You found loose change in the return slot. +25 Points.",
		PointsDelta: 25,
		PoolDelta:   0,
	}, nil
}

// handleGrail gives the user 50% of the pool
func handleGrail(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get guild to access pool
	guild, err := guildService.GetGuildInfo(s, db, guildID, "")
	if err != nil {
		return nil, err
	}

	// Calculate 50% of pool
	poolWin := guild.Pool * 0.5

	// Return result - DrawCard will apply the PoolDelta
	return &models.CardResult{
		Message:     "You discovered the Holy Grail! You won 50% of the pool!",
		PointsDelta: poolWin,
		PoolDelta:   -poolWin,
	}, nil
}

// handlePettyTheft requires user selection, returns special result
func handlePettyTheft(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "Petty Theft requires you to select a target!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

// ExecutePickpocketSteal executes the actual steal after user selection
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

	// Steal 5% of target's points
	stealAmount := amount

	// Don't steal more than target has (shouldn't happen, but safety check)
	if stealAmount > targetUser.Points {
		stealAmount = targetUser.Points
	}

	// Update points
	user.Points += stealAmount
	targetUser.Points -= stealAmount

	// Save both users
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

// handleNilFee makes the user pay 50 points to the pool
func handleNilFee(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user to check current points (after card cost was deducted)
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

	// Calculate the cost that was paid (CardDrawCount was already incremented before handler is called)
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

// handleTipJar forces the person above you on the leaderboard to give you 10 points
func handleTipJar(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get current user to find their position
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	// Find user above on leaderboard (higher points, or same points but lower ID if tied)
	var userAbove models.User
	result := db.Where("guild_id = ? AND (points > ? OR (points = ? AND id < ?))", guildID, user.Points, user.Points, user.ID).
		Order("points DESC, id ASC").
		First(&userAbove)

	if result.Error != nil || userAbove.ID == 0 {
		// No one above you on leaderboard
		return &models.CardResult{
			Message:     "You're at the top of the leaderboard! No one to tip you. The card fizzles out.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	// Transfer 10 points (DrawCard will handle the actual updates)
	transferAmount := 10.0
	if userAbove.Points < transferAmount {
		transferAmount = userAbove.Points // Can't transfer more than they have
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

// handleHoleInPocket makes the user lose 5% of their total points to the Pool
func handleHoleInPocket(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user to calculate 5% of their points
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	// Calculate 5% loss
	lossAmount := user.Points * 0.05

	// Round to 1 decimal place
	lossAmount = float64(int(lossAmount*10+0.5)) / 10.0

	// Don't lose more than user has
	if lossAmount > user.Points {
		lossAmount = user.Points
	}

	return &models.CardResult{
		Message:     fmt.Sprintf("You found a hole in your pocket! Lost %.1f points (5%% of your total) to the Pool.", lossAmount),
		PointsDelta: -lossAmount,
		PoolDelta:   lossAmount,
	}, nil
}

// handlePiggyBank gives the user 5% of their total points from the void
func handlePiggyBank(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user to calculate 5% of their points
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	// Calculate 5% gain
	gainAmount := user.Points * 0.05

	// Round to 1 decimal place
	gainAmount = float64(int(gainAmount*10+0.5)) / 10.0

	return &models.CardResult{
		Message:     fmt.Sprintf("You broke open your piggy bank! Gained %.1f points (5%% of your total) from the void.", gainAmount),
		PointsDelta: gainAmount,
		PoolDelta:   0,
	}, nil
}

// handleParticipationTrophy gives the user 1 point
func handleParticipationTrophy(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You received a participation trophy! +1 Point and a pat on the back.",
		PointsDelta: 1,
		PoolDelta:   0,
	}, nil
}

// handleTimeout prevents the user from drawing another card for 1 hour
func handleTimeout(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user to set timeout
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	// Set timeout to 1 hour from now
	timeoutUntil := time.Now().Add(2 * time.Hour)
	user.CardDrawTimeoutUntil = &timeoutUntil

	// Save the timeout (this is part of the transaction in DrawCard)
	if err := db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &models.CardResult{
		Message:     "You've been timed out! You cannot buy another card for 1 hour.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

// handleBadInvestment makes the user lose 50 points
func handleBadInvestment(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user to check current points (after card cost was deducted)
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
		Message:     fmt.Sprintf("Your bad investment cost you dearly. -%.0f Points.", deductAmount),
		PointsDelta: -deductAmount,
		PoolDelta:   0,
	}, nil
}

// handleFoundWallet gives the user 50 points
func handleFoundWallet(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You found a wallet on the ground! +50 Points.",
		PointsDelta: 50,
		PoolDelta:   0,
	}, nil
}

// handleCharityCase gives 75 points if user is in bottom 50% of players
func handleCharityCase(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get current user
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

	// Check if user is in bottom 50%
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

// handleTaxSeason makes user lose 75 points if they are in top 50% of players
func handleTaxSeason(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get current user
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
			Message:     "No other players found. Tax season skipped you.",
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
			Message:     "Could not determine your position. Tax season passed you by.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil
	}

	// Check if user is in top 50%
	totalPlayers := len(allUsers)
	top50PercentThreshold := totalPlayers / 2
	if totalPlayers%2 != 0 {
		top50PercentThreshold = (totalPlayers + 1) / 2
	}

	isTop50Percent := userPosition <= top50PercentThreshold

	if isTop50Percent {
		// Don't deduct more than user has (points already loaded above)
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

// handleLuckyHorseshoe adds the card to the user's inventory
// The discount will be applied when they draw their next card
func handleLuckyHorseshoe(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You found a lucky horseshoe! Your next card purchase will cost half price.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

// handleUnluckyCat adds the card to the user's inventory
// The penalty will be applied when they draw their next card
func handleUnluckyCat(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "An unlucky cat crossed your path! Your next card purchase will cost double.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

// handlePickpocketCommon steals 50 points from a random active user
func handlePickpocketCommon(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get all users in guild except the current user
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

	// Pick a random user
	randomIndex := rand.Intn(len(allUsers))
	targetUser := allUsers[randomIndex]

	// Steal 50 points (DrawCard will handle the actual updates)
	stealAmount := 50.0
	if targetUser.Points < stealAmount {
		stealAmount = targetUser.Points // Can't steal more than they have
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

// handleDroppedLoot gives 50 points to a random active user
func handleDroppedLoot(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get current user to check points
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	// Get all users in guild except the current user
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

	// Pick a random user
	randomIndex := rand.Intn(len(allUsers))
	targetUser := allUsers[randomIndex]

	// Give 50 points (DrawCard will handle the actual updates)
	giveAmount := 50.0
	// Ensure user doesn't go below 0 (DrawCard will also check, but we adjust here for accuracy)
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

// handleScrapingBy gives the user 20 points
func handleScrapingBy(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "You're scraping by! +20 Points.",
		PointsDelta: 20,
		PoolDelta:   0,
	}, nil
}

// handleRust makes the user lose 20 points
func handleRust(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user to check current points (after card cost was deducted)
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	// Don't deduct more than user has
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

// handleMinorGlitch gives the user 1-100 points (randomized)
func handleMinorGlitch(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Generate random number between 1 and 100
	randomPoints := float64(rand.Intn(100) + 1)

	return &models.CardResult{
		Message:     fmt.Sprintf("A minor glitch occurred! You gained %.0f points.", randomPoints),
		PointsDelta: randomPoints,
		PoolDelta:   0,
	}, nil
}

// handleHighFive gives both the user and a random active user 10 points
func handleHighFive(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get all users in guild except the current user
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

	// Both users gain 10 points
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

// handleRickRoll makes the user lose 5 points and includes a YouTube link
func handleRickRoll(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user to check current points (after card cost was deducted)
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}

	// Don't deduct more than user has
	deductAmount := 5.0
	if user.Points < deductAmount {
		deductAmount = user.Points
	}

	// Include YouTube link in the message - Discord will auto-preview it
	youtubeLink := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	return &models.CardResult{
		Message:     fmt.Sprintf("You got rick rolled! -%.0f Points.\n\n%s", deductAmount, youtubeLink),
		PointsDelta: -deductAmount,
		PoolDelta:   0,
	}, nil
}

// handlePocketSand refunds the cost of drawing the card
func handlePocketSand(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user and guild to determine card cost
	var user models.User
	var guild models.Guild
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return nil, err
	}

	// Calculate the cost that was paid (CardDrawCount was already incremented before handler is called)
	var refundAmount float64
	switch user.CardDrawCount {
	case 1:
		refundAmount = guild.CardDrawCost
	case 2:
		refundAmount = guild.CardDrawCost * 10
	default:
		refundAmount = guild.CardDrawCost * 100
	}

	// Refund the cost (remove from pool and give back to user)
	return &models.CardResult{
		Message:     fmt.Sprintf("It's very effective! Refunded %.0f points (the cost of this card).", refundAmount),
		PointsDelta: refundAmount,
		PoolDelta:   -refundAmount, // Remove from pool
	}, nil
}

// handleShield adds the Shield to the user's inventory (already handled by AddToInventory)
// and informs the user that the next negative effect against them will be blocked.
func handleShield(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "A shimmering barrier surrounds you. Your next negative effect against you will be blocked.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

// handleMajorGlitch gives 100 points to everyone in the server
func handleMajorGlitch(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get all users in the guild
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

	// Update all users except the current user (current user will get points via PointsDelta)
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
		PointsDelta: gainAmount, // Current user gets points through normal flow
		PoolDelta:   0,
	}, nil
}

// handleDoubleDown adds the Double Down card to the user's inventory (already handled by AddToInventory)
// The next winning bet payout will be doubled
func handleDoubleDown(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "Your next winning bet payout will be doubled! (Does not apply to parlays)",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

// handleEmotionalHedge adds the card to the user's inventory (already handled by AddToInventory)
// It provides a refund if the subscribed team loses straight up
func handleEmotionalHedge(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:     "Emotional Hedge active! If your subscribed team loses straight up on your next bet, you get 50% refund.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

// handleJester requires user selection to mute them
func handleJester(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "The Jester requires you to select a target to mute!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

// ExecuteJesterMute applies a 15-minute timeout to the target user
func ExecuteJesterMute(s *discordgo.Session, db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
	// 15 minutes from now
	timeoutUntil := time.Now().Add(15 * time.Minute)

	// Apply the timeout using Discord's native feature
	// This disables sending messages, reacting, and speaking in voice
	err := s.GuildMemberTimeout(guildID, targetUserID, &timeoutUntil)
	if err != nil {
		// If it fails (e.g., target is Admin or Bot has low permissions), return a friendly error
		return &models.CardResult{
			Message:     "Failed to mute the target! They might be an Admin or too powerful.",
			PointsDelta: 0,
			PoolDelta:   0,
		}, nil // Return nil error so the bot doesn't crash, just shows the message
	}

	targetID := targetUserID
	return &models.CardResult{
		Message:           "The Jester laughs! Target has been muted for 15 minutes.",
		PointsDelta:       0,
		PoolDelta:         0,
		TargetUserID:      &targetID,
		TargetPointsDelta: 0,
	}, nil
}

// handleGenerousDonation adds the card to the user's inventory (already handled by AddToInventory)
// It also pre-charges the cost of a standard card draw (level 1 cost) if the user can afford it.
// The actual logic for applying this to another user is in cardOrch.go
func handleGenerousDonation(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user and guild to determine cost
	var user models.User
	var guild models.Guild
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return nil, err
	}
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return nil, err
	}

	// Check if user has Shield - if so, Shield blocks Generous Donation and card fizzles
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, guildID, ShieldCardID).
		Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count > 0 {
		// Shield blocks it - remove both Generous Donation (just added) and Shield
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

	// Cost of a standard (level 1) card draw
	standardCost := guild.CardDrawCost

	var message string
	var pointsDelta float64
	var poolDelta float64

	// If user can afford it, charge them now
	if user.Points >= standardCost {
		pointsDelta = -standardCost
		poolDelta = standardCost // Add to pool immediately
		message = fmt.Sprintf("You have generously paid %.0f points forward! The next user to draw a standard cost card will get it for free.", standardCost)
	} else {
		// If the user cannot afford the donation, remove the card from their inventory so it doesn't trigger the free draw effect.
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

// removeCardFromInventory removes a specific card from the user's inventory (one instance)
func removeCardFromInventory(db *gorm.DB, userID uint, guildID string, cardID int) error {
	// Find one instance
	var item models.UserInventory
	result := db.Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cardID).First(&item)
	if result.Error != nil {
		return result.Error
	}
	// Delete it
	return db.Delete(&item).Error
}

// handleStimulusCheck gives 50 points to everyone in the server
func handleStimulusCheck(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get all users in the guild
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

// handleFactoryReset resets the user's points to 1000 if they have less than 1000 points
func handleFactoryReset(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	// Get user to check current points
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
		Message:     "Factory Reset triggered, but you have 1000 or more points. Nothing changed.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}

// handleQuickFlip flips a coin. Heads: Double your card cost back. Tails: Get nothing.
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

	// Calculate the cost that was paid (CardDrawCount was already incremented before handler is called)
	var cardCost float64
	switch user.CardDrawCount {
	case 1:
		cardCost = guild.CardDrawCost
	case 2:
		cardCost = guild.CardDrawCost * 10
	default:
		cardCost = guild.CardDrawCost * 100
	}

	// Flip a coin: 0 = tails, 1 = heads
	coinFlip := rand.Intn(2)

	if coinFlip == 1 {
		// Heads: Double your card cost back
		winnings := cardCost * 2
		return &models.CardResult{
			Message:     fmt.Sprintf("Heads! You doubled your card cost back and gained %.0f points!", winnings),
			PointsDelta: winnings,
			PoolDelta:   0,
		}, nil
	}

	// Tails: Get nothing
	return &models.CardResult{
		Message:     "Tails! You get nothing.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, nil
}
