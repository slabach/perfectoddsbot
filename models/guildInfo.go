package models

import "gorm.io/gorm"

type Guild struct {
	gorm.Model
	ID                    uint `gorm:"primaryKey"`
	GuildID               string
	GuildName             string
	BetChannelID          string
	PointsPerMessage      float64
	StartingPoints        float64
	PremiumEnabled        bool
	SubscribedTeam        *string
	Pool                  float64 `gorm:"default:0"`
	CardDrawCost          float64 `gorm:"default:50"`
	CardDrawCooldownMinutes int  `gorm:"default:60"`
}
