package models

import (
	"time"

	"gorm.io/gorm"
)

type Guild struct {
	gorm.Model
	ID                      uint `gorm:"primaryKey"`
	GuildID                 string
	GuildName               string
	BetChannelID            string
	PointsPerMessage        float64
	StartingPoints          float64
	PremiumEnabled          bool
	SubscribedTeam          *string
	Pool                    float64 `gorm:"default:0"`
	CardDrawCost            float64 `gorm:"default:10"`
	CardDrawCooldownMinutes int     `gorm:"default:60"`
	CardDrawingEnabled      bool    `gorm:"default:true"`
	PoolDrainUntil          *time.Time
	EmperorActiveUntil      *time.Time
	EmperorHolderDiscordID  *string
	TotalCardDraws          int `gorm:"default:0"`
	LastEpicDrawAt          int `gorm:"default:0"`
	LastMythicDrawAt        int `gorm:"default:0"`

	// Expansions
	TarotExpansion      bool `gorm:"default:true"`
	CollegiateExpansion bool `gorm:"default:true"`
}
