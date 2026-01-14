package models

import (
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

// CardHandler is a function that executes when a card is drawn
// It receives the Discord session, database, user ID, and guild ID
// Returns a CardResult containing the effects of the card
type CardHandler func(s *discordgo.Session, db *gorm.DB, userID string, guildID string) (*CardResult, error)

// Card represents a single playing card with its properties
type Card struct {
	ID                   int
	Name                 string
	Description          string
	Rarity               string // "Common", "Rare", "Epic", "Mythic"
	Weight               int    // For RNG weighted distribution
	Handler              CardHandler
	AddToInventory       bool         // If true, card is added to user's inventory when drawn
	RoyaltyDiscordUserID *string      // Optional user ID for royalty payments
	RequiredSubscription bool         // If true, card can only be drawn by guilds with a subscribed team
	Options              []CardOption // If true, card is a choice card
}

type CardOption struct {
	ID          int
	Name        string
	Description string
}

// CardResult contains the outcome of executing a card
type CardResult struct {
	Message           string  // Message to display to user
	PointsDelta       float64 // Points gained/lost by the user who drew the card
	PoolDelta         float64 // Points added/subtracted from the pool
	TargetUserID      *string // Optional target user (for steals, transfers, etc.)
	TargetPointsDelta float64 // Points gained/lost by target user
	RequiresSelection bool    // If true, card needs user interaction (e.g., Pickpocket)
	SelectionType     string  // Type of selection needed (e.g., "user" for user select menu)
}
