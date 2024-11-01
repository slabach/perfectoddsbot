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

	result := db.Where("paid = 0 AND active = 1 AND cfbd_id IS NOT NULL").Find(&betList)
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
			t := bet.GameStartDate.In(est)

			if currentTimeEST.After(t) {

				bet.Active = false
				db.Save(&bet)

				_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
					ID:         *bet.MessageID,
					Channel:    bet.ChannelID,
					Components: &[]discordgo.MessageComponent{},
				})
				if err != nil {
					fmt.Println(err)
				}

				var secondaryMsgs []models.BetMessage
				secondaryResult := db.Where("active = 1 AND bet_id = ?", bet.ID).Find(&secondaryMsgs)
				if secondaryResult.Error != nil {
					continue
				}
				if len(secondaryMsgs) > 0 {
					for _, msg := range secondaryMsgs {
						msg.Active = false
						db.Save(&msg)

						_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
							ID:         *msg.MessageID,
							Channel:    msg.ChannelID,
							Components: &[]discordgo.MessageComponent{},
						})
						if err != nil {
							continue
						}
					}
				}
			}
		}
	}

	return nil
}
