package models

import (
	"gorm.io/gorm"
	"time"
)

type Bet struct {
	gorm.Model
	ID            uint `gorm:"primaryKey"`
	Description   string
	Option1       string
	Option2       string
	Odds1         int
	Odds2         int
	Active        bool
	Paid          bool `gorm:"default:false"`
	GuildID       string
	BetsOption1   int
	BetsOption2   int
	MessageID     *string
	ChannelID     string
	CfbdID        *string
	EspnID        *string
	GameStartDate *time.Time
	AdminCreated  bool
	Spread        *float64
}
