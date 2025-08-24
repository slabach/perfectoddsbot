package scheduler_jobs

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/models/external"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/extService"
	"perfectOddsBot/services/guildService"
	"strconv"
)

func CheckGameEnd(s *discordgo.Session, db *gorm.DB) error {
	var dbBetList []models.Bet

	result := db.Where("paid = 0 AND active = 0 AND (cfbd_id IS NOT NULL OR espn_id IS NOT NULL)").Find(&dbBetList)
	if result.Error != nil {
		return result.Error
	}

	// check the count of each first. if there are no CFB bets, we dont need to get CFB games (and vice versa)
	cbbCount := 0
	cfbCount := 0
	for _, cBet := range dbBetList {
		if cBet.CfbdID != nil {
			cfbCount++
		}
		if cBet.EspnID != nil {
			cbbCount++
		}
	}

	var cfbdList []external.CFBD_BettingLines
	if cfbCount > 0 {
		cfbGameList, err := extService.GetCFBGames()
		if err != nil {
			common.SendError(s, nil, err, db)
		}
		cfbdList = cfbGameList
	}

	var espnList []external.ESPN_Event
	if cbbCount > 0 {
		cbbGameList, err := extService.GetCbbGames()
		if err != nil {
			return err
		}
		espnList = cbbGameList
	}

	cbbBetMap := make(map[string]external.ESPN_Event)
	for _, obj := range espnList {
		cbbBetMap[obj.ID] = obj
	}

	cfbBetMap := make(map[int]external.CFBD_BettingLines)
	for _, obj := range cfbdList {
		cfbBetMap[obj.ID] = obj
	}

	for _, bet := range dbBetList {
		if bet.CfbdID != nil {
			betCfbdId, _ := strconv.Atoi(*bet.CfbdID)
			if obj, found := cfbBetMap[betCfbdId]; found {
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

					err := ResolveCFBBBet(s, bet, db)
					if err != nil {
						return err
					}
				}
			}
		}
		if bet.EspnID != nil {
			betEspnId := *bet.EspnID
			if obj, found := cbbBetMap[betEspnId]; found {
				if obj.Status.Type.Name == "STATUS_FINAL" {
					var betEntries []models.BetEntry
					entriesResult := db.Where("bet_id = ?", bet.ID).Find(&betEntries)
					if entriesResult.RowsAffected == 0 {
						bet.Paid = true
						db.Save(&bet)
						continue
					}

					homeTeam := external.ESPN_Competitor{}
					awayTeam := external.ESPN_Competitor{}

					for _, comp := range obj.Competitions[0].Competitors {
						if comp.HomeAway == "home" {
							homeTeam = comp
						}
						if comp.HomeAway == "away" {
							awayTeam = comp
						}
					}

					homeScore, _ := strconv.Atoi(homeTeam.Score)
					awayScore, _ := strconv.Atoi(awayTeam.Score)
					scoreDiff := homeScore - awayScore
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

					err := ResolveCFBBBet(s, bet, db)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func ResolveCFBBBet(s *discordgo.Session, bet models.Bet, db *gorm.DB) error {
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

		betOption := common.GetSchoolName(bet.Option1)
		spread := *entry.Spread
		if entry.Option == 2 {
			spread = *entry.Spread * -1
			betOption = common.GetSchoolName(bet.Option2)
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
			loserList += fmt.Sprintf("%s - Bet: %s %s - **Lost $%d**\n", username, betOption, common.FormatOdds(spread), entry.Amount)
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
