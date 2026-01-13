package cards

import (
	"perfectOddsBot/models"
	"perfectOddsBot/services/guildService"

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
	return &models.CardResult{
		Message:     "You cut your finger drawing the card. -10 Points.",
		PointsDelta: -10,
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

// handlePickpocket requires user selection, returns special result
func handlePickpocket(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*models.CardResult, error) {
	return &models.CardResult{
		Message:           "The Pickpocket requires you to select a target!",
		PointsDelta:       0,
		PoolDelta:         0,
		RequiresSelection: true,
		SelectionType:     "user",
	}, nil
}

// ExecutePickpocketSteal executes the actual steal after user selection
func ExecutePickpocketSteal(db *gorm.DB, userID string, targetUserID string, guildID string) (*models.CardResult, error) {
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
	stealAmount := targetUser.Points * 0.05

	// Round to 1 decimal place
	stealAmount = float64(int(stealAmount*10+0.5)) / 10.0

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
