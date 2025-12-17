package models

import (
	"time"

	"gorm.io/gorm"
)

type Migration struct {
	gorm.Model
	Name       string `gorm:"uniqueIndex; size:255"`
	ExecutedAt time.Time
}
