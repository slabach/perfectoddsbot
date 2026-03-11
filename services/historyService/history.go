package historyService

import (
	"encoding/json"
	"log"
	"perfectOddsBot/models"

	"gorm.io/gorm"
)

func RecordCardPlayHistory(db *gorm.DB, guildID string, targetUserID string, targetUserDBID uint, cardID uint, cardName string, playedByUserID string, pointsBefore float64, pointsAfter float64, pointsDelta float64, handCardsGained []string, handCardsLost []string, betsResolved []uint) error {

	var gainedJSON, lostJSON, betsJSON []byte
	var err error

	if len(handCardsGained) > 0 {
		gainedJSON, err = json.Marshal(handCardsGained)
		if err != nil {
			log.Printf("Error marshaling handCardsGained: %v", err)
			return err
		}
	}

	if len(handCardsLost) > 0 {
		lostJSON, err = json.Marshal(handCardsLost)
		if err != nil {
			log.Printf("Error marshaling handCardsLost: %v", err)
			return err
		}
	}

	if len(betsResolved) > 0 {
		betsJSON, err = json.Marshal(betsResolved)
		if err != nil {
			log.Printf("Error marshaling betsResolved: %v", err)
			return err
		}
	}

	history := models.CardPlayHistory{
		GuildID:         guildID,
		TargetUserID:    targetUserID,
		TargetUserDBID:  targetUserDBID,
		CardID:          cardID,
		CardName:        cardName,
		PlayedByUserID:  playedByUserID,
		PointsBefore:    pointsBefore,
		PointsAfter:     pointsAfter,
		PointsDelta:     pointsDelta,
		HandCardsGained: string(gainedJSON),
		HandCardsLost:   string(lostJSON),
		BetsResolved:    string(betsJSON),
	}

	result := db.Create(&history)
	if result.Error != nil {
		log.Printf("Error creating card play history: %v", result.Error)
		return result.Error
	}

	return nil
}
