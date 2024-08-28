package models

type User struct {
	ID        uint   `gorm:"primaryKey"`
	DiscordID string `gorm:"uniqueIndex:user_guild_idx"`
	GuildID   string `gorm:"uniqueIndex:user_guild_idx"`
	Points    int
}
