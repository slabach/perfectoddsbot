package models

import "gorm.io/gorm"

type Parlay struct {
	gorm.Model
	ID            uint `gorm:"primaryKey"`
	UserID        uint
	User          User `gorm:"foreignKey:UserID"`
	GuildID       string
	Amount        int
	TotalOdds     float64 // Combined odds multiplier
	Status        string  // "pending", "won", "lost", "partial" (if any bet lost before all resolved)
	ParlayEntries []ParlayEntry
}

type ParlayEntry struct {
	gorm.Model
	ID             uint `gorm:"primaryKey"`
	ParlayID       uint
	Parlay         Parlay `gorm:"foreignKey:ParlayID"`
	BetID          uint
	Bet            Bet   `gorm:"foreignKey:BetID"`
	SelectedOption int   // 1 or 2, which option the user selected for this bet
	Spread         *float64 // Spread at the time the parlay entry was created (nil for moneyline bets)
	Resolved       bool  `gorm:"default:false"` // Whether this individual bet has been resolved
	Won            *bool // true if won, false if lost, nil if not resolved yet
}
