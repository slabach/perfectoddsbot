package models

type BetEntry struct {
	ID     uint `gorm:"primaryKey"`
	UserID uint
	BetID  uint
	Bet    Bet `gorm:"foreignKey:BetID"`
	Option int
	Amount int
}
