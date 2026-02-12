package scheduler_jobs

import (
	"log"
	cardService "perfectOddsBot/services/cardService"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func RefreshDeck(s *discordgo.Session, db *gorm.DB) error {
	log.Println("Refreshing deck from database...")
	err := cardService.LoadDeckFromDB(db)
	if err != nil {
		log.Printf("Error refreshing deck: %v", err)
		return err
	}
	log.Println("Deck refreshed successfully")
	return nil
}
