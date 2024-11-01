package models

type BetMessage struct {
	ID        uint `gorm:"primaryKey"`
	BetID     uint
	Bet       Bet `gorm:"foreignKey:BetID"`
	Active    bool
	MessageID *string
	ChannelID string
}
