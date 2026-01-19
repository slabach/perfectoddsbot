package models

import "gorm.io/gorm"

type Parlay struct {
	gorm.Model
	ID            uint `gorm:"primaryKey"`
	UserID        uint
	User          User `gorm:"foreignKey:UserID"`
	GuildID       string
	Amount        int
	TotalOdds     float64
	Status        string
	ParlayEntries []ParlayEntry
}

type ParlayEntry struct {
	gorm.Model
	ID             uint `gorm:"primaryKey"`
	ParlayID       uint
	Parlay         Parlay `gorm:"foreignKey:ParlayID"`
	BetID          uint
	Bet            Bet `gorm:"foreignKey:BetID"`
	SelectedOption int
	Spread         *float64
	Resolved       bool `gorm:"default:false"`
	Won            *bool
}
