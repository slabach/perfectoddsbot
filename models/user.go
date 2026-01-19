package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	ID                 uint   `gorm:"primaryKey"`
	DiscordID          string `gorm:"uniqueIndex:user_guild_idx; size:64"`
	GuildID            string `gorm:"uniqueIndex:user_guild_idx; size:64"`
	Points             float64
	Username           *string
	TotalBetsWon       int        `gorm:"default:0"`
	TotalBetsLost      int        `gorm:"default:0"`
	TotalPointsWon     float64    `gorm:"default:0"`
	TotalPointsLost    float64    `gorm:"default:0"`
	FirstCardDrawCycle *time.Time // Timestamp when current draw cycle started
	CardDrawCount      int        `gorm:"default:0"` // Number of draws in current cycle
	CardDrawTimeoutUntil *time.Time // Timestamp when card draw timeout expires (nil if not timed out)
	BetLockoutUntil    *time.Time // Timestamp when bet lockout expires (nil if not locked out)
}
