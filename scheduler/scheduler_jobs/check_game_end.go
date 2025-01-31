package scheduler_jobs

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/models/external"
	"perfectOddsBot/services/cfbdService"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"strconv"
)

func CheckGameEnd(s *discordgo.Session, db *gorm.DB) error {
	var dbBetList []models.Bet

	result := db.Where("paid = 0 AND active = 0 AND cfbd_id IS NOT NULL").Find(&dbBetList)
	if result.Error != nil {
		return result.Error
	}
	fmt.Println(dbBetList)

	cfbdList, err := cfbdService.GetCFBGames()
	if err != nil {
		return err
	}

	betMap := make(map[int]external.CFBD_BettingLines)
	for _, obj := range cfbdList {
		betMap[obj.ID] = obj
	}

	for _, bet := range dbBetList {
		betCfbdId, _ := strconv.Atoi(*bet.CfbdID)
		if obj, found := betMap[betCfbdId]; found {
			fmt.Println(obj.ID)
			if obj.HomeScore != nil && obj.AwayScore != nil {
				scoreDiff := *obj.HomeScore - *obj.AwayScore

				var betEntries []models.BetEntry
				entriesResult := db.Where("bet_id = ?", bet.ID).Find(&betEntries)
				if entriesResult.RowsAffected == 0 {
					bet.Paid = true
					db.Save(&bet)
					continue
				}

				for _, entry := range betEntries {
					// 1 = home team beat spread
					// 2 = away team beat spread
					spreadWinner := 2
					if *entry.Spread < float64(0) {
						// eg. Spread -9 (home 9 point favored)
						// scoreDiff > 9
						if float64(scoreDiff) > -(*entry.Spread) {
							spreadWinner = 1
						}
					} else {
						// eg. Spread 9 (home 9 point underdog)
						// scoreDiff >= -9
						if float64(scoreDiff) >= -(*entry.Spread) {
							spreadWinner = 1
						}
					}

					if entry.Option == spreadWinner {
						entry.AutoCloseWin = true
						db.Save(&entry)
					}
				}

				err = ResolveCFBBet(s, bet, db)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func ResolveCFBBet(s *discordgo.Session, bet models.Bet, db *gorm.DB) error {
	winnersList := ""
	loserList := ""
	guild, err := guildService.GetGuildInfo(s, db, bet.GuildID, bet.ChannelID)
	if err != nil {
		return err
	}

	var entries []models.BetEntry
	db.Where("bet_id = ?", bet.ID).Find(&entries)

	totalPayout := 0.0
	for _, entry := range entries {
		var user models.User
		db.First(&user, "id = ?", entry.UserID)
		if user.ID == 0 {
			continue
		}
		username := common.GetUsername(s, user.GuildID, user.DiscordID)

		betOption := "Home"
		spread := *entry.Spread
		if entry.Option == 2 {
			spread = *entry.Spread * -1
			betOption = "Away"
		}

		if entry.AutoCloseWin {
			payout := common.CalculatePayout(entry.Amount, entry.Option, bet)
			user.Points += payout
			db.Save(&user)
			totalPayout += payout

			if payout > 0 {
				winnersList += fmt.Sprintf("%s - Bet: %s %s - **Won $%.1f**\n", username, betOption, common.FormatOdds(spread), payout)
			}
		} else {
			loserList += fmt.Sprintf("%s - Bet: %s %s - **Lost $%d**\n", username, betOption, common.FormatOdds(*entry.Spread*-1), entry.Amount)
		}
	}

	bet.Active = false
	db.Save(&bet)
	db.Model(&bet).UpdateColumn("paid", true).UpdateColumn("active", false)

	response := fmt.Sprintf("Bet '%s' has been resolved!\nTotal payout: **%.1f** points.\n**Winners:**\n%s\n**Losers:**\n%s", bet.Description, totalPayout, winnersList, loserList)
	_, err = s.ChannelMessageSendComplex(guild.BetChannelID, &discordgo.MessageSend{
		Content: response,
	})
	if err != nil {
		return err
	}

	return nil
}
