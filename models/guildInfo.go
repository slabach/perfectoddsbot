package models

type Guild struct {
	ID           uint `gorm:"primaryKey"`
	GuildID      string
	BetChannelID string
}
