package models

import (
	"time"

	"gorm.io/gorm"
)

type CardPlayHistory struct {
	ID             uint           `gorm:"primaryKey"`
	GuildID        string         `gorm:"index:idx_guild_target_created"`
	TargetUserID   string         `gorm:"index:idx_guild_target_created"`
	TargetUserDBID uint           `gorm:"index"`
	CardID         uint           `gorm:"index"`
	CardName       string         `gorm:"size:255"`
	PlayedByUserID string         `gorm:"size:255"`
	PointsBefore   float64        `gorm:"type:decimal(20,2)"`
	PointsAfter    float64        `gorm:"type:decimal(20,2)"`
	PointsDelta    float64        `gorm:"type:decimal(20,2)"`
	HandCardsGained string        `gorm:"type:json"` // JSON array of card names
	HandCardsLost   string        `gorm:"type:json"` // JSON array of card names
	BetsResolved    string        `gorm:"type:json"` // JSON array of bet IDs
	CreatedAt      time.Time      `gorm:"index:idx_guild_target_created"`
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}
