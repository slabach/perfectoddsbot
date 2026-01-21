package models

import (
	"gorm.io/gorm"
)

type UserInventory struct {
	gorm.Model
	ID           uint   `gorm:"primaryKey"`
	UserID       uint   `gorm:"index:idx_user_guild_card; not null"`
	GuildID      string `gorm:"index:idx_user_guild_card; size:64; not null"`
	CardID       uint   `gorm:"index:idx_user_guild_card; not null"`
	Card         Card   `gorm:"foreignKey:CardID;"`
	TargetBetID  *uint
	TargetUserID *string `gorm:"size:64"`
	BetAmount    float64
	User         User `gorm:"foreignKey:UserID; constraint:OnDelete:CASCADE"`
}
