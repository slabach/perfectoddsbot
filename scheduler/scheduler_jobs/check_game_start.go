package scheduler_jobs

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"time"
)

func CheckGameStart(s *discordgo.Session, db *gorm.DB) error {
	var betList []models.Bet

	result := db.Where("paid = 0 AND active = 0 AND api_id IS NOT NULL").Find(&betList)
	if result.Error != nil {
		return result.Error
	}

	for _, bet := range betList {
		est, err := time.LoadLocation("America/New_York")
		if err != nil {
			return err
		}

		// Get the current time in EST
		currentTimeEST := time.Now().In(est)

		if bet.GameStartDate != nil {
			if currentTimeEST.Before(*bet.GameStartDate) {

				bet.Active = false
				db.Save(&bet)

				_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
					ID:         *bet.MessageID,
					Channel:    bet.ChannelID,
					Components: &[]discordgo.MessageComponent{},
				})
				if err != nil {
					return fmt.Errorf("error removing buttons from the message: %v", err)
				}
			}
		}
	}

	return nil
}
