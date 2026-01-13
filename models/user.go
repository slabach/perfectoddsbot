package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	ID             uint       `gorm:"primaryKey"`
	DiscordID      string     `gorm:"uniqueIndex:user_guild_idx; size:64"`
	GuildID        string     `gorm:"uniqueIndex:user_guild_idx; size:64"`
	Points         float64
	Username       *string
	TotalBetsWon   int        `gorm:"default:0"`
	TotalBetsLost  int        `gorm:"default:0"`
	TotalPointsWon float64    `gorm:"default:0"`
	TotalPointsLost float64   `gorm:"default:0"`
	LastCardDraw   *time.Time
}
