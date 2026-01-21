package models

import (
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

type CardHandler func(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*CardResult, error)

type Card struct {
	gorm.Model
	ID                   uint `gorm:"primaryKey"`
	Code                 string
	Name                 string
	Description          string
	Rarity               string
	RarityID             uint
	CardRarity           CardRarity `gorm:"foreignKey:RarityID"`
	Weight               int
	Handler              CardHandler `gorm:"-"`
	HandlerName          string
	AddToInventory       bool    `gorm:"default:false"`
	RoyaltyDiscordUserID *string `gorm:"size:64"`
	RequiredSubscription bool    `gorm:"default:false"`
	Options              []CardOption
	UserPlayable         bool `gorm:"default:false"`
	Active               bool `gorm:"default:true"`
}

type CardOption struct {
	gorm.Model
	ID          uint `gorm:"primaryKey"`
	CardID      uint `gorm:"index"`
	Name        string
	Description string
}

type CardRarity struct {
	gorm.Model
	ID      uint `gorm:"primaryKey"`
	Name    string
	Weight  int
	Color   string
	Icon    string
	Royalty float64
}

type CardResult struct {
	ID                uint
	Message           string
	PointsDelta       float64
	PoolDelta         float64
	TargetUserID      *string
	TargetPointsDelta float64
	RequiresSelection bool
	SelectionType     string
}
