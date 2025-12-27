package scheduler_jobs

import (
	"fmt"
	"log"
	"perfectOddsBot/models"
	"perfectOddsBot/models/external"
	"perfectOddsBot/services/betService"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/extService"
	"perfectOddsBot/services/guildService"
	"perfectOddsBot/services/messageService"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

// calculateBetEntryWin determines if a bet entry wins based on the option, score difference, and spread.
// Parameters:
//   - option: 1 for home team + spread, 2 for away team - spread
//   - scoreDiff: homeScore - awayScore
//   - spread: spread value stored from home team's perspective
//   - If away team is favored by 3.5, spread = +3.5
//   - If home team is favored by 3.5, spread = -3.5
//
// Returns true if the bet entry wins, false otherwise.
func calculateBetEntryWin(option int, scoreDiff int, spread float64) bool {
	if option == 1 {
		// Option 1: homeTeam + spread wins if (homeScore + spread) > awayScore
		// i.e., if scoreDiff > -spread
		return float64(scoreDiff) > -spread
	} else {
		// Option 2: awayTeam - spread wins if (awayScore - spread) > homeScore
		// i.e., if (awayScore - homeScore) > spread
		// i.e., if -scoreDiff > spread
		return float64(-scoreDiff) > spread
	}
}

func CheckGameEnd(s *discordgo.Session, db *gorm.DB) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in CheckGameEnd", r)
			debug.PrintStack()
			err = fmt.Errorf("panic recovered in CheckGameEnd: %v", r)
		}
	}()

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
					}

					for _, entry := range betEntries {
						var won bool
						if bet.Spread == nil {
							// Moneyline bet: winner is determined by actual game result
							// Option 1 is home team, Option 2 is away team
							if entry.Option == 1 {
								// Home team wins if home score > away score
								won = scoreDiff > 0
							} else {
								// Away team wins if away score > home score
								won = scoreDiff < 0
							}
						} else {
							// ATS bet: use spread-based calculation
							spread := *entry.Spread
							won = calculateBetEntryWin(entry.Option, scoreDiff, spread)
						}

						if won {
							entry.AutoCloseWin = true
							db.Save(&entry)
						}
					}

					resolveErr := ResolveCFBBBet(s, bet, db)
					if resolveErr != nil {
						return resolveErr
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

					// Robustly match Option 1 to the correct competitor by name
					// instead of assuming Option 1 is always the "home" team from the API.
					op1Name := common.GetSchoolName(bet.Option1)
					var score1, score2 int
					var matched bool

					for _, comp := range obj.Competitions[0].Competitors {
						// Check if this competitor matches Option 1's name
						if comp.Team.ShortDisplayName == op1Name {
							score1, _ = strconv.Atoi(comp.Score)
							matched = true
						} else {
							// If it's not Option 1, it's Option 2
							score2, _ = strconv.Atoi(comp.Score)
						}
					}

					// Fallback to legacy logic if name matching fails (e.g. name change)
					if !matched {
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
						score1, _ = strconv.Atoi(homeTeam.Score) // Assume Option 1 is Home
						score2, _ = strconv.Atoi(awayTeam.Score) // Assume Option 2 is Away
					}

					// scoreDiff is now relative to Option 1: (Option1Score - Option2Score)
					scoreDiff := score1 - score2
					for _, entry := range betEntries {
						var won bool
						if bet.Spread == nil {
							// Moneyline bet: winner is determined by actual game result
							// Option 1 wins if score1 > score2, Option 2 wins if score2 > score1
							if entry.Option == 1 {
								won = scoreDiff > 0
							} else {
								won = scoreDiff < 0
							}
						} else {
							// ATS bet: use spread-based calculation
							spread := *entry.Spread
							won = calculateBetEntryWin(entry.Option, scoreDiff, spread)
						}

						if won {
							entry.AutoCloseWin = true
							db.Save(&entry)
						}
					}

					resolveErr := ResolveCFBBBet(s, bet, db)
					if resolveErr != nil {
						return resolveErr
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
	lostPoolAmount := 0.0
	for _, entry := range entries {
		var user models.User
		db.First(&user, "id = ?", entry.UserID)
		if user.ID == 0 {
			continue
		}
		username := common.GetUsernameWithDB(db, s, user.GuildID, user.DiscordID)

		betOption := common.GetSchoolName(bet.Option1)
		var spreadDisplay string
		if bet.Spread == nil {
			// Moneyline bet: no spread to display
			spreadDisplay = ""
		} else {
			spread := *entry.Spread
			if entry.Option == 2 {
				spread = *entry.Spread * -1
			}
			spreadDisplay = common.FormatOdds(spread)
		}
		if entry.Option == 2 {
			betOption = common.GetSchoolName(bet.Option2)
		}

		if entry.AutoCloseWin {
			payout := common.CalculatePayout(entry.Amount, entry.Option, bet)
			user.Points += payout
			user.TotalBetsWon++
			user.TotalPointsWon += payout
			db.Save(&user)
			totalPayout += payout

			if payout > 0 {
				if spreadDisplay != "" {
					winnersList += fmt.Sprintf("%s - Bet: %s %s - **Won $%.1f**\n", username, betOption, spreadDisplay, payout)
				} else {
					winnersList += fmt.Sprintf("%s - Bet: %s - **Won $%.1f**\n", username, betOption, payout)
				}
			}
		} else {
			user.TotalBetsLost++
			user.TotalPointsLost += float64(entry.Amount)
			db.Save(&user)
			lostPoolAmount += float64(entry.Amount)
			if spreadDisplay != "" {
				loserList += fmt.Sprintf("%s - Bet: %s %s - **Lost $%d**\n", username, betOption, spreadDisplay, entry.Amount)
			} else {
				loserList += fmt.Sprintf("%s - Bet: %s - **Lost $%d**\n", username, betOption, entry.Amount)
			}
		}
	}

	// Add lost bet amounts to guild pool (atomic update to prevent race conditions)
	if lostPoolAmount > 0 {
		db.Model(&models.Guild{}).Where("id = ?", guild.ID).UpdateColumn("pool", gorm.Expr("pool + ?", lostPoolAmount))
	}

	bet.Active = false
	db.Save(&bet)
	db.Model(&bet).UpdateColumn("paid", true).UpdateColumn("active", false)

	// Determine winning option for parlay updates
	// Check which option has winning entries
	winningOption := 0
	for _, entry := range entries {
		if entry.AutoCloseWin {
			winningOption = entry.Option
			break // All winning entries should have the same option
		}
	}

	// Update parlays if we determined a winning option
	if winningOption > 0 {
		updateErr := betService.UpdateParlaysOnBetResolution(s, db, bet.ID, winningOption)
		if updateErr != nil {
			// Log error but don't fail the bet resolution
			log.Printf("Error updating parlays for bet %d: %v\n", bet.ID, updateErr)
		}
	}

	embed := messageService.BuildBetResolutionEmbed(
		bet.Description,
		"",
		totalPayout,
		strings.TrimSpace(winnersList),
		strings.TrimSpace(loserList),
	)
	_, err = s.ChannelMessageSendComplex(guild.BetChannelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		return err
	}

	return nil
}
