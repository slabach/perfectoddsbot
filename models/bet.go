package models

type Bet struct {
	ID          uint `gorm:"primaryKey"`
	Description string
	Option1     string
	Option2     string
	Odds1       int
	Odds2       int
	Active      bool
	Paid        bool `gorm:"default:false"`
	GuildID     string
	BetsOption1 int
	BetsOption2 int
	MessageID   string
	ChannelID   string
}
