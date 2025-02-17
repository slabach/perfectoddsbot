package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	ID        uint   `gorm:"primaryKey"`
	DiscordID string `gorm:"uniqueIndex:user_guild_idx; size:64"`
	GuildID   string `gorm:"uniqueIndex:user_guild_idx; size:64"`
	Points    float64
	Username  *string
}
