package models

import (
	"gorm.io/gorm"
)

type ErrorLog struct {
	gorm.Model
	ID      uint   `gorm:"primaryKey"`
	GuildID string `gorm:"size:64"`
	Message string
}
