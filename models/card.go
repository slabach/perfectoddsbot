package models

import (
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

type CardHandler func(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*CardResult, error)

type Card struct {
	ID                   int
	Name                 string
	Description          string
	Rarity               string
	Weight               int
	Handler              CardHandler
	AddToInventory       bool
	RoyaltyDiscordUserID *string
	RequiredSubscription bool
	Options              []CardOption
}

type CardOption struct {
	ID          int
	Name        string
	Description string
}

type CardResult struct {
	Message           string
	PointsDelta       float64
	PoolDelta         float64
	TargetUserID      *string
	TargetPointsDelta float64
	RequiresSelection bool
	SelectionType     string
}
