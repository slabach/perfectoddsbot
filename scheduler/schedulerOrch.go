package scheduler

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/scheduler/scheduler_jobs"
	scheduled_cards "perfectOddsBot/scheduler/scheduler_jobs/scheduled_cards"

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

	// Card expiration jobs. All card checks should be run every hour.
	_, err = cronService.AddFunc("0 0 */1 * * *", func() {
		// Soft-delete inventory rows past expires_at (Vampire, Devil, Redshirt, Home Field Advantage, etc.)
		err := scheduled_cards.CheckExpiredInventory(s, db)
		if err != nil {
			fmt.Println(err)
		}

		// Runs every hour to collect Loan Shark debts
		err = scheduled_cards.CheckLoanShark(s, db)
		if err != nil {
			fmt.Println(err)
		}

		// Runs every hour to process active Leech cards and expire them after 12 hours
		err = scheduled_cards.CheckLeech(s, db)
		if err != nil {
			fmt.Println(err)
		}

		// Runs every hour to process The Hanged Man cards after 24 hours
		err = scheduled_cards.CheckHangedMan(s, db)
		if err != nil {
			fmt.Println(err)
		}

		// Runs every hour to refresh the deck from the database
		err = scheduled_cards.RefreshDeck(s, db)
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
