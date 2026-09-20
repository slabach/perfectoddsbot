package scheduler_jobs

import (
	"fmt"
	"log"
	"perfectOddsBot/models"
	"perfectOddsBot/services/messageService"
	"runtime/debug"
	"time"
	_ "time/tzdata"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func CheckGameStart(s *discordgo.Session, db *gorm.DB) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in CheckGameStart", r)
			debug.PrintStack()
			err = fmt.Errorf("panic recovered in CheckGameStart: %v", r)
		}
	}()

	var betList []models.Bet

	result := db.Where("paid = 0 AND active = 1 AND (cfbd_id IS NOT NULL OR espn_id IS NOT NULL)").Find(&betList)
	if result.Error != nil {
		return result.Error
	}

	for _, bet := range betList {
		est, err := time.LoadLocation("America/New_York")
		if err != nil {
			return err
		}

		currentTimeEST := time.Now().In(est)

		if bet.GameStartDate != nil {
			t := bet.GameStartDate.In(est)

			if currentTimeEST.After(t) {
				bet.Active = false
				db.Save(&bet)

				messageService.CloseBetMessages(s, db, bet, "📢 Bet has been CLOSED (Will Auto Resolve)")
			}
		}
	}

	return nil
}
