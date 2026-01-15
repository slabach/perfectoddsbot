package scheduler

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/scheduler/scheduler_jobs"

	"github.com/bwmarrin/discordgo"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

func SetupCron(s *discordgo.Session, db *gorm.DB) {
	cronService := cron.New(cron.WithSeconds())

	_, err := cronService.AddFunc("0 0 */1 * 8-12 *", func() {
		// // Every hour, August through December
		err := scheduler_jobs.CheckGameEnd(s, db)
		if err != nil {
			fmt.Println(err)
		}
	})
	_, err = cronService.AddFunc("0 0 */1 * 1-5 *", func() {
		// // Every hour, January through May
		err := scheduler_jobs.CheckGameEnd(s, db)
		if err != nil {
			fmt.Println(err)
		}
	})

	_, err = cronService.AddFunc("0 0 9 * 8-12 *", func() {
		// // At 9am every day, August through December
		err := scheduler_jobs.CheckCFBLines(s, db)
		if err != nil {
			fmt.Println(err)
		}

		err = scheduler_jobs.CheckSubscribedCFBTeam(s, db)
		if err != nil {
			fmt.Println(err)
		}
	})
	_, err = cronService.AddFunc("0 0 9 * 1-2 *", func() {
		// // At 9am every day, January through February
		err := scheduler_jobs.CheckCFBLines(s, db)
		if err != nil {
			fmt.Println(err)
		}

		err = scheduler_jobs.CheckSubscribedCFBTeam(s, db)
		if err != nil {
			fmt.Println(err)
		}
	})

	_, err = cronService.AddFunc("0 */5 * * 8-12 *", func() {
		// // Every 5 minutes, August through December
		err := scheduler_jobs.CheckGameStart(s, db)
		if err != nil {
			fmt.Println(err)
		}
	})
	_, err = cronService.AddFunc("0 */5 * * 1-5 *", func() {
		// // Every 5 minutes, January through May
		err := scheduler_jobs.CheckGameStart(s, db)
		if err != nil {
			fmt.Println(err)
		}
	})

	_, err = cronService.AddFunc("0 0 */1 * 10-12 *", func() {
		// // Every hour, October through December
		err := scheduler_jobs.CheckSubscribedCBBTeam(s, db)
		if err != nil {
			fmt.Println(err)
		}
	})
	_, err = cronService.AddFunc("0 0 */1 * 1-5 *", func() {
		// // Every hour, January through May
		err := scheduler_jobs.CheckSubscribedCBBTeam(s, db)
		if err != nil {
			fmt.Println(err)
		}
	})

	_, err = cronService.AddFunc("0 0 0 */1 * *", func() {
		// Runs every day to collect Loan Shark debts
		err := scheduler_jobs.CheckLoanShark(s, db)
		if err != nil {
			fmt.Println(err)
		}
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
