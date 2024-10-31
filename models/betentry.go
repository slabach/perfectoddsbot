package models

type BetEntry struct {
	ID     uint `gorm:"primaryKey"`
	User   User `gorm:"foreignKey:UserID"`
	UserID uint `gorm:"foreignKey:"`
	BetID  uint
	Bet    Bet `gorm:"foreignKey:BetID"`
	Option int
	Amount int
}
