package models

import (
	"gorm.io/gorm"
	"time"
)

type Migration struct {
	gorm.Model
	ID         uint      `gorm:"primaryKey"`
	Name       string    `gorm:"uniqueIndex; size:255"`
	ExecutedAt time.Time
}

