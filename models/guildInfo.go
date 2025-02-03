package models

type Guild struct {
	ID               uint `gorm:"primaryKey"`
	GuildID          string
	GuildName        string
	BetChannelID     string
	PointsPerMessage float64
	StartingPoints   float64
	PremiumEnabled   bool
	SubscribedTeam   *string
}
