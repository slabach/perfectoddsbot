package models

type User struct {
	ID        uint   `gorm:"primaryKey"`
	DiscordID string `gorm:"uniqueIndex:user_guild_idx; size:64"`
	GuildID   string `gorm:"uniqueIndex:user_guild_idx; size:64"`
	Points    int
	Username  *string
}
