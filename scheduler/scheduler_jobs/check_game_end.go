package scheduler_jobs

import (
	"fmt"
	"log"
	"perfectOddsBot/models"
	"perfectOddsBot/models/external"
	"perfectOddsBot/services/betService"
	cardService "perfectOddsBot/services/cardService"
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

func CheckGameEnd(s *discordgo.Session, db *gorm.DB) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in CheckGameEnd", r)
			debug.PrintStack()
			err = fmt.Errorf("panic recovered in CheckGameEnd: %v", r)
		}
	}()

	var dbBetList []models.Bet

	result := db.Where("paid = 0 AND active = 0 AND (cfbd_id IS NOT NULL OR espn_id IS NOT NULL) AND deleted_at IS NULL").Find(&dbBetList)
	if result.Error != nil {
		return result.Error
	}

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

					// Determine winning option even if there are no bet entries (for parlay updates)
					winningOption := 0
					if bet.Spread == nil {
						// Moneyline bet: winner is determined by actual game result
						// Option 1 is home team, Option 2 is away team
						if scoreDiff > 0 {
							winningOption = 1 // Home team wins
						} else if scoreDiff < 0 {
							winningOption = 2 // Away team wins
						}
					} else {
						// ATS bet: determine winner based on spread
						// For ATS, we need to check which option would win
						// Option 1 is home team + spread, Option 2 is away team - spread
						if common.CalculateBetEntryWin(1, scoreDiff, *bet.Spread) {
							winningOption = 1
						} else {
							winningOption = 2
						}
					}

					if entriesResult.RowsAffected == 0 {
						if winningOption > 0 {
							updateErr := betService.UpdateParlaysOnBetResolution(s, db, bet.ID, winningOption, scoreDiff)
							if updateErr != nil {
								log.Printf("Error updating parlays for bet %d: %v\n", bet.ID, updateErr)
							}
						}
						bet.Paid = true
						bet.Active = false
						db.Save(&bet)
					} else {
						for _, entry := range betEntries {
							var won bool
							if bet.Spread == nil {
								if entry.Option == 1 {
									won = scoreDiff > 0
								} else {
									won = scoreDiff < 0
								}
							} else {
								var spread float64
								if entry.Spread != nil {
									spread = *entry.Spread
								} else if bet.Spread != nil {
									spread = *bet.Spread
								} else {
									spread = 0.0
								}
								won = common.CalculateBetEntryWin(entry.Option, scoreDiff, spread)
							}

							if won {
								entry.AutoCloseWin = true
								db.Save(&entry)
							}
						}

						resolveErr := ResolveCFBBBet(s, bet, db, winningOption, scoreDiff)
						if resolveErr != nil {
							return resolveErr
						}
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

					op1Name := common.GetSchoolName(bet.Option1)
					var score1, score2 int
					var matched bool

					for _, comp := range obj.Competitions[0].Competitors {
						if comp.Team.ShortDisplayName == op1Name {
							score1, _ = strconv.Atoi(comp.Score)
							matched = true
						} else {
							score2, _ = strconv.Atoi(comp.Score)
						}
					}

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
						score1, _ = strconv.Atoi(homeTeam.Score)
						score2, _ = strconv.Atoi(awayTeam.Score)
					}

					scoreDiff := score1 - score2

					winningOption := 0
					if bet.Spread == nil {
						if scoreDiff > 0 {
							winningOption = 1
						} else if scoreDiff < 0 {
							winningOption = 2
						}
					} else {
						if common.CalculateBetEntryWin(1, scoreDiff, *bet.Spread) {
							winningOption = 1
						} else {
							winningOption = 2
						}
					}

					if entriesResult.RowsAffected == 0 {
						if winningOption > 0 {
							updateErr := betService.UpdateParlaysOnBetResolution(s, db, bet.ID, winningOption, scoreDiff)
							if updateErr != nil {
								log.Printf("Error updating parlays for bet %d: %v\n", bet.ID, updateErr)
							}
						}
						bet.Paid = true
						bet.Active = false
						db.Save(&bet)
						continue
					}

					for _, entry := range betEntries {
						var won bool
						if bet.Spread == nil {
							if entry.Option == 1 {
								won = scoreDiff > 0
							} else {
								won = scoreDiff < 0
							}
						} else {
							var spread float64
							if entry.Spread != nil {
								spread = *entry.Spread
							} else if bet.Spread != nil {
								spread = *bet.Spread
							} else {
								spread = 0.0
							}
							won = common.CalculateBetEntryWin(entry.Option, scoreDiff, spread)
						}

						if won {
							entry.AutoCloseWin = true
							db.Save(&entry)
						}
					}

					resolveErr := ResolveCFBBBet(s, bet, db, winningOption, scoreDiff)
					if resolveErr != nil {
						return resolveErr
					}
				}
			}
		}
	}

	return nil
}

func ResolveCFBBBet(s *discordgo.Session, bet models.Bet, db *gorm.DB, winningOption int, scoreDiff int) error {
	winnersList := ""
	loserList := ""
	guild, err := guildService.GetGuildInfo(s, db, bet.GuildID, bet.ChannelID)
	if err != nil {
		return err
	}

	var entries []models.BetEntry
	db.Where("bet_id = ?", bet.ID).Find(&entries)

	totalPayout := 0.0
	totalWinningPayouts := 0.0
	winnerDiscordIDs := make(map[string]float64) // Track DiscordID -> total payout amount
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
			// Check if entry.Spread is nil (legacy entries) and fall back to bet.Spread
			var spread float64
			if entry.Spread != nil {
				spread = *entry.Spread
			} else {
				spread = *bet.Spread
			}
			if entry.Option == 2 {
				spread = spread * -1
			}
			spreadDisplay = common.FormatOdds(spread)
		}
		if entry.Option == 2 {
			betOption = common.GetSchoolName(bet.Option2)
		}

		if entry.AutoCloseWin {
			payout := common.CalculatePayout(entry.Amount, entry.Option, bet)

			unoApplied, isWinAfterUno, err := cardService.ApplyUnoReverseIfApplicable(db, user, bet.ID, true)
			if err != nil {
				log.Printf("Error checking Uno Reverse: %v", err)
			}

			if unoApplied && !isWinAfterUno {
				user.TotalBetsLost++
				user.TotalPointsLost += float64(entry.Amount)
				db.Save(&user)

				if spreadDisplay != "" {
					loserList += fmt.Sprintf("%s - Bet: %s %s - **Lost $%d** (Uno Reverse!)\n", username, betOption, spreadDisplay, entry.Amount)
				} else {
					loserList += fmt.Sprintf("%s - Bet: %s - **Lost $%d** (Uno Reverse!)\n", username, betOption, entry.Amount)
				}
				continue
			}

			_, _, antiAntiBetLosers, antiAntiBetApplied, err := cardService.ApplyAntiAntiBetIfApplicable(db, user, true)
			if err != nil {
				log.Printf("Error checking Anti-Anti-Bet (Win): %v", err)
			}
			if antiAntiBetApplied {
				if len(antiAntiBetLosers) > 0 {
					for _, loser := range antiAntiBetLosers {
						cardHolderUsername := common.GetUsernameWithDB(db, s, user.GuildID, loser.DiscordID)
						loserList += fmt.Sprintf("%s - **Lost $%.1f** (Anti-Anti-Bet!)\n", cardHolderUsername, loser.Payout)
					}
				}
			}

			// Define card consumer closure
			consumer := func(db *gorm.DB, user models.User, cardID int) error {
				return cardService.PlayCardFromInventory(s, db, user, cardID)
			}

			// Check for Double Down card and apply 2x multiplier if available
			modifiedPayout, hasDoubleDown, err := cardService.ApplyDoubleDownIfAvailable(db, consumer, user, payout)
			if err != nil {
				log.Printf("Error checking Double Down for auto-resolved bet: %v", err)
				// Continue with original payout if error
				modifiedPayout = payout
				hasDoubleDown = false
			}

			// Store payout before applying Gambler to check if it was doubled
			payoutAfterDoubleDown := modifiedPayout

			// Check for Gambler card and apply 50/50 chance to double if available
			modifiedPayout, hasGambler, err := cardService.ApplyGamblerIfAvailable(db, consumer, user, modifiedPayout, true)
			if err != nil {
				log.Printf("Error checking Gambler for auto-resolved bet: %v", err)
				hasGambler = false
			}

			// Check for Emotional Hedge
			hedgeRefund, hedgeApplied, err := cardService.ApplyEmotionalHedgeIfApplicable(db, consumer, user, bet, entry.Option, float64(entry.Amount), scoreDiff)
			if err != nil {
				log.Printf("Error checking Emotional Hedge: %v", err)
			}

			// Check for Bet Insurance (fizzle on win)
			_, insuranceApplied, err := cardService.ApplyBetInsuranceIfApplicable(db, consumer, user, 0, true)
			if err != nil {
				log.Printf("Error checking Bet Insurance (Win): %v", err)
			}

			user.Points += modifiedPayout
			user.TotalBetsWon++
			user.TotalPointsWon += modifiedPayout

			if hedgeApplied && hedgeRefund > 0 {
				user.Points += hedgeRefund
			}

			db.Save(&user)
			totalPayout += modifiedPayout + hedgeRefund
			totalWinningPayouts += modifiedPayout + hedgeRefund
			winnerDiscordIDs[user.DiscordID] += modifiedPayout + hedgeRefund // Accumulate payout for this winner

			if modifiedPayout > 0 {
				doubleDownMsg := ""
				if hasDoubleDown {
					doubleDownMsg = " (Double Down: 2x payout!)"
				}
				gamblerMsg := ""
				if hasGambler {
					if modifiedPayout > payoutAfterDoubleDown {
						gamblerMsg = " (The Gambler: 2x payout!)"
					} else {
						gamblerMsg = " (The Gambler: consumed, no double)"
					}
				}
				hedgeMsg := ""
				if hedgeApplied && hedgeRefund > 0 {
					hedgeMsg = fmt.Sprintf(" (Emotional Hedge: Refunding $%.1f)", hedgeRefund)
				} else if hedgeApplied {
					hedgeMsg = " (Emotional Hedge: consumed)"
				}
				insuranceMsg := ""
				if insuranceApplied {
					insuranceMsg = " (Bet Insurance: consumed)"
				}

				if spreadDisplay != "" {
					winnersList += fmt.Sprintf("%s - Bet: %s %s - **Won $%.1f**%s%s%s%s\n", username, betOption, spreadDisplay, modifiedPayout, doubleDownMsg, gamblerMsg, hedgeMsg, insuranceMsg)
				} else {
					winnersList += fmt.Sprintf("%s - Bet: %s - **Won $%.1f**%s%s%s%s\n", username, betOption, modifiedPayout, doubleDownMsg, gamblerMsg, hedgeMsg, insuranceMsg)
				}
			}
		} else {
			// Uno Reverse Check (Loss -> Win)
			unoApplied, isWinAfterUno, err := cardService.ApplyUnoReverseIfApplicable(db, user, bet.ID, false)
			if err != nil {
				log.Printf("Error checking Uno Reverse: %v", err)
			}

			if unoApplied && isWinAfterUno {
				// Flipped to WIN
				payout := common.CalculatePayout(entry.Amount, entry.Option, bet)

				// Define card consumer closure
				consumer := func(db *gorm.DB, user models.User, cardID int) error {
					return cardService.PlayCardFromInventory(s, db, user, cardID)
				}

				// Check for Double Down card and apply 2x multiplier if available
				modifiedPayout, hasDoubleDown, err := cardService.ApplyDoubleDownIfAvailable(db, consumer, user, payout)
				if err != nil {
					log.Printf("Error checking Double Down for auto-resolved bet (Uno Reverse): %v", err)
					modifiedPayout = payout
					hasDoubleDown = false
				}

				// Store payout before applying Gambler to check if it was doubled
				payoutAfterDoubleDown := modifiedPayout

				// Check for Gambler card and apply 50/50 chance to double if available
				modifiedPayout, hasGambler, err := cardService.ApplyGamblerIfAvailable(db, consumer, user, modifiedPayout, true)
				if err != nil {
					log.Printf("Error checking Gambler for auto-resolved bet (Uno Reverse): %v", err)
					hasGambler = false
				}

				// Check for Emotional Hedge
				hedgeRefund, hedgeApplied, err := cardService.ApplyEmotionalHedgeIfApplicable(db, consumer, user, bet, entry.Option, float64(entry.Amount), scoreDiff)
				if err != nil {
					log.Printf("Error checking Emotional Hedge (Uno Reverse): %v", err)
				}

				// Check for Bet Insurance (fizzle on win)
				_, insuranceApplied, err := cardService.ApplyBetInsuranceIfApplicable(db, consumer, user, 0, true)
				if err != nil {
					log.Printf("Error checking Bet Insurance (Win/Uno Reverse): %v", err)
				}

				user.Points += modifiedPayout
				user.TotalBetsWon++
				user.TotalPointsWon += modifiedPayout

				if hedgeApplied && hedgeRefund > 0 {
					user.Points += hedgeRefund
				}

				db.Save(&user)
				totalPayout += modifiedPayout + hedgeRefund
				totalWinningPayouts += modifiedPayout + hedgeRefund
				winnerDiscordIDs[user.DiscordID] += modifiedPayout + hedgeRefund // Accumulate payout for this winner

				if modifiedPayout > 0 {
					doubleDownMsg := ""
					if hasDoubleDown {
						doubleDownMsg = " (Double Down: 2x payout!)"
					}
					gamblerMsg := ""
					if hasGambler {
						if modifiedPayout > payoutAfterDoubleDown {
							gamblerMsg = " (The Gambler: 2x payout!)"
						} else {
							gamblerMsg = " (The Gambler: consumed, no double)"
						}
					}
					hedgeMsg := ""
					if hedgeApplied && hedgeRefund > 0 {
						hedgeMsg = fmt.Sprintf(" (Emotional Hedge: Refunding $%.1f)", hedgeRefund)
					} else if hedgeApplied {
						hedgeMsg = " (Emotional Hedge: consumed)"
					}
					insuranceMsg := ""
					if insuranceApplied {
						insuranceMsg = " (Bet Insurance: consumed)"
					}

					if spreadDisplay != "" {
						winnersList += fmt.Sprintf("%s - Bet: %s %s - **Won $%.1f** (Uno Reverse!)%s%s%s%s\n", username, betOption, spreadDisplay, modifiedPayout, doubleDownMsg, gamblerMsg, hedgeMsg, insuranceMsg)
					} else {
						winnersList += fmt.Sprintf("%s - Bet: %s - **Won $%.1f** (Uno Reverse!)%s%s%s%s\n", username, betOption, modifiedPayout, doubleDownMsg, gamblerMsg, hedgeMsg, insuranceMsg)
					}
				}
				continue
			}

			antiAntiBetPayout, antiAntiBetWinners, _, antiAntiBetApplied, err := cardService.ApplyAntiAntiBetIfApplicable(db, user, false)
			if err != nil {
				log.Printf("Error checking Anti-Anti-Bet (Loss): %v", err)
			}
			if antiAntiBetApplied && antiAntiBetPayout > 0 {
				totalPayout += antiAntiBetPayout
				for _, winner := range antiAntiBetWinners {
					cardHolderUsername := common.GetUsernameWithDB(db, s, user.GuildID, winner.DiscordID)
					winnersList += fmt.Sprintf("%s - **Won $%.1f** (Anti-Anti-Bet!)\n", cardHolderUsername, winner.Payout)
				}
			}

			consumer := func(db *gorm.DB, user models.User, cardID int) error {
				return cardService.PlayCardFromInventory(s, db, user, cardID)
			}

			hedgeRefund, hedgeApplied, err := cardService.ApplyEmotionalHedgeIfApplicable(db, consumer, user, bet, entry.Option, float64(entry.Amount), scoreDiff)
			if err != nil {
				log.Printf("Error checking Emotional Hedge: %v", err)
			}

			insuranceRefund, insuranceApplied, err := cardService.ApplyBetInsuranceIfApplicable(db, consumer, user, float64(entry.Amount), false)
			if err != nil {
				log.Printf("Error checking Bet Insurance (Loss): %v", err)
			}

			// Check for Gambler card and apply 50/50 chance to double loss if available
			lossAmount := float64(entry.Amount)
			modifiedLoss, hasGambler, err := cardService.ApplyGamblerIfAvailable(db, consumer, user, -lossAmount, false)
			if err != nil {
				log.Printf("Error checking Gambler (Loss): %v", err)
				hasGambler = false
			}

			// If gambler doubled the loss, modifiedLoss will be double the negative loss
			// So we need to convert back and add extra to pool
			actualLoss := lossAmount
			if hasGambler && modifiedLoss < -lossAmount {
				// Loss was doubled (modifiedLoss is more negative)
				actualLoss = -modifiedLoss
			}

			user.TotalBetsLost++
			user.TotalPointsLost += actualLoss

			if hedgeApplied && hedgeRefund > 0 {
				user.Points += hedgeRefund
			}

			if insuranceApplied && insuranceRefund > 0 {
				user.Points += insuranceRefund
			}

			db.Save(&user)

			hedgeMsg := ""
			if hedgeApplied && hedgeRefund > 0 {
				hedgeMsg = fmt.Sprintf(" (Emotional Hedge: Refunding $%.1f)", hedgeRefund)
			} else if hedgeApplied {
				hedgeMsg = " (Emotional Hedge: consumed)"
			}

			insuranceMsg := ""
			if insuranceApplied && insuranceRefund > 0 {
				insuranceMsg = fmt.Sprintf(" (Bet Insurance: Refunding $%.1f)", insuranceRefund)
			} else if insuranceApplied {
				insuranceMsg = " (Bet Insurance: consumed)"
			}

			gamblerMsg := ""
			if hasGambler {
				if actualLoss > lossAmount {
					gamblerMsg = " (The Gambler: 2x loss!)"
				} else {
					gamblerMsg = " (The Gambler: consumed, no double)"
				}
			}

			if spreadDisplay != "" {
				loserList += fmt.Sprintf("%s - Bet: %s %s - **Lost $%.0f**%s%s%s\n", username, betOption, spreadDisplay, actualLoss, hedgeMsg, insuranceMsg, gamblerMsg)
			} else {
				loserList += fmt.Sprintf("%s - Bet: %s - **Lost $%.0f**%s%s%s\n", username, betOption, actualLoss, hedgeMsg, insuranceMsg, gamblerMsg)
			}
		}
	}

	if totalWinningPayouts > 0 {
		vampirePayout, vampireWinners, vampireApplied, err := cardService.ApplyVampireIfApplicable(db, bet.GuildID, totalWinningPayouts, winnerDiscordIDs)
		if err != nil {
			log.Printf("Error checking Vampire: %v", err)
		} else if vampireApplied && vampirePayout > 0 {
			totalPayout += vampirePayout
			if len(vampireWinners) > 0 {
				for _, winner := range vampireWinners {
					cardHolderUsername := common.GetUsernameWithDB(db, s, bet.GuildID, winner.DiscordID)
					winnersList += fmt.Sprintf("%s - **Won $%.1f** (Vampire)\n", cardHolderUsername, winner.Payout)
				}
			}
		}
	}

	bet.Active = false
	db.Save(&bet)
	db.Model(&bet).UpdateColumn("paid", true).UpdateColumn("active", false)

	if winningOption > 0 {
		updateErr := betService.UpdateParlaysOnBetResolution(s, db, bet.ID, winningOption, scoreDiff)
		if updateErr != nil {
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
