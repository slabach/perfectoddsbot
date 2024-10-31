package scheduler

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/scheduler/scheduler_jobs"
)

func SetupCron(s *discordgo.Session, db *gorm.DB) {
	cronService := cron.New(cron.WithSeconds())

	_, err := cronService.AddFunc("0 0 */1 * 8-12 *", func() {
		// // Every hour, August through December
		err := scheduler_jobs.CheckLines(db)
		fmt.Println(err)
	})
	_, err = cronService.AddFunc("0 0 */1 * 8-12 *", func() {
		// // Every hour, January through February
		err := scheduler_jobs.CheckLines(db)
		fmt.Println(err)
	})

	_, err = cronService.AddFunc("0 */1 * * 8-12 *", func() {
		// // Every minute, August through December
		err := scheduler_jobs.CheckGameStart(s, db)
		fmt.Println(err)
	})
	_, err = cronService.AddFunc("0 */1 * * 8-12 *", func() {
		// // Every minute, January through February
		err := scheduler_jobs.CheckGameStart(s, db)
		fmt.Println(err)
	})

	if err != nil {
		errLog := models.ErrorLog{
			GuildID: "CRON ERR",
			Message: fmt.Sprintf("%v", err),
		}
		db.Create(&errLog)
	}

	cronService.Start()
}
