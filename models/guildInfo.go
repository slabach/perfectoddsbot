package models

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Guild struct {
	gorm.Model
	ID                      uint `gorm:"primaryKey"`
	GuildID                 string
	GuildName               string
	BetChannelID            string
	PointsPerMessage        float64
	StartingPoints          float64
	PremiumEnabled          bool
	SubscribedTeam          *string
	Pool                    float64 `gorm:"default:0"`
	CardDrawCost            float64 `gorm:"default:10"`
	CardDrawCooldownMinutes int     `gorm:"default:60"`
	CardDrawingEnabled      bool    `gorm:"default:true"`
	RestrictedDrawEnabled   bool    `gorm:"default:false"`
	RestrictedDrawUserIDs   string  `gorm:"type:json"`
	RestrictedDrawCardIDs   string  `gorm:"type:json"`
	PoolDrainUntil          *time.Time
	EmperorActiveUntil      *time.Time
	EmperorHolderDiscordID  *string
	TotalCardDraws          int `gorm:"default:0"`
	LastEpicDrawAt          int `gorm:"default:0"`
	LastMythicDrawAt        int `gorm:"default:0"`

	// Expansions
	TarotExpansion      bool `gorm:"default:true"`
	CollegiateExpansion bool `gorm:"default:true"`
}

func (g *Guild) normalizeRestrictedDrawJSONFields() {
	if g == nil {
		return
	}
	if strings.TrimSpace(g.RestrictedDrawUserIDs) == "" {
		g.RestrictedDrawUserIDs = "[]"
	}
	if strings.TrimSpace(g.RestrictedDrawCardIDs) == "" {
		g.RestrictedDrawCardIDs = "[]"
	}
}

func (g *Guild) BeforeCreate(tx *gorm.DB) error {
	g.normalizeRestrictedDrawJSONFields()
	return nil
}

func (g *Guild) BeforeSave(tx *gorm.DB) error {
	g.normalizeRestrictedDrawJSONFields()
	return nil
}

func (g *Guild) RestrictedDrawUserIDsSlice() ([]string, error) {
	if g == nil || g.RestrictedDrawUserIDs == "" {
		return []string{}, nil
	}

	var userIDs []string
	if err := json.Unmarshal([]byte(g.RestrictedDrawUserIDs), &userIDs); err != nil {
		return []string{}, err
	}

	return userIDs, nil
}

func (g *Guild) RestrictedDrawCardIDsSlice() ([]uint, error) {
	if g == nil || g.RestrictedDrawCardIDs == "" {
		return []uint{}, nil
	}

	var cardIDs []uint
	if err := json.Unmarshal([]byte(g.RestrictedDrawCardIDs), &cardIDs); err != nil {
		return []uint{}, err
	}

	return cardIDs, nil
}
