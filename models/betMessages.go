package models

import "gorm.io/gorm"

type BetMessage struct {
	gorm.Model
	ID        uint `gorm:"primaryKey"`
	BetID     uint
	Bet       Bet `gorm:"foreignKey:BetID"`
	Active    bool
	MessageID *string
	ChannelID string
}
