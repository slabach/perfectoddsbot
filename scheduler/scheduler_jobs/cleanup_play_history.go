package scheduler_jobs

import (
	"log"
	"perfectOddsBot/models"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func CleanupPlayHistory(s *discordgo.Session, db *gorm.DB) {
	log.Println("Running cleanup play history job...")

	// Delete records older than 3 days
	cutoffTime := time.Now().AddDate(0, 0, -3)

	result := db.Unscoped().Where("created_at < ?", cutoffTime).Delete(&models.CardPlayHistory{})
	if result.Error != nil {
		log.Printf("Error cleaning up play history: %v", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		log.Printf("Cleaned up %d old card play history records.", result.RowsAffected)
	}
}
