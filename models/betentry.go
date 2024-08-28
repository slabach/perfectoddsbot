package models

type BetEntry struct {
	ID     uint `gorm:"primaryKey"`
	UserID uint
	BetID  uint
	Option int
	Amount int
}
