package models

import "gorm.io/gorm"

type Guild struct {
	gorm.Model
	ID               uint `gorm:"primaryKey"`
	GuildID          string
	GuildName        string
	BetChannelID     string
	PointsPerMessage float64
	StartingPoints   float64
	PremiumEnabled   bool
	SubscribedTeam   *string
}
