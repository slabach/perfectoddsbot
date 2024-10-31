package models

type ErrorLog struct {
	ID      uint   `gorm:"primaryKey"`
	GuildID string `gorm:"size:64"`
	Message string
}
