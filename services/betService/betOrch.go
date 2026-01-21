package betService

import (
	"fmt"
	"strings"

	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"perfectOddsBot/services/messageService"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func CreateCustomBet(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	if !common.IsAdmin(s, i) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You are not authorized to use this command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}
		return
	}

	options := i.ApplicationCommandData().Options
	description := options[0].StringValue()
	option1 := options[1].StringValue()
	option2 := options[2].StringValue()
	odds1 := -110
	odds2 := -110
	if len(options) > 3 {
		if options[3] != nil {
			odds1 = int(options[3].IntValue())
		}
		if len(options) > 4 && options[4] != nil {
			odds2 = int(options[4].IntValue())
		}
	}
	guildID := i.GuildID

	bet := models.Bet{
		Description:  description,
		Option1:      option1,
		Option2:      option2,
		Odds1:        odds1,
		Odds2:        odds2,
		Active:       true,
		GuildID:      guildID,
		ChannelID:    i.ChannelID,
		AdminCreated: true,
	}
	db.Create(&bet)

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprint("📢 New Bet Created"),
		Description: description,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  fmt.Sprintf("1️⃣ %s", option1),
				Value: fmt.Sprintf("Odds: %s", common.FormatOdds(float64(odds1))),
			},
			{
				Name:  fmt.Sprintf("2️⃣ %s", option2),
				Value: fmt.Sprintf("Odds: %s", common.FormatOdds(float64(odds2))),
			},
		},
		Color: 0x3498db,
	}

	buttons := messageService.GetAllButtonList(s, i, option1, option2, bet.ID)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: buttons,
				},
			},
		},
	})

	msg, err := s.InteractionResponse(i.Interaction)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	if bet.MessageID == nil {
		bet.MessageID = &msg.ID
		db.Save(&bet)
	}

	return
}

func ResolveBetByID(s *discordgo.Session, i *discordgo.InteractionCreate, betID int, winningOption int, db *gorm.DB) {
	var bet models.Bet
	winnersList := ""
	loserList := ""
	result := db.First(&bet, "id = ? AND guild_id = ?", betID, i.GuildID)
	if result.Error != nil || bet.ID == 0 {
		response := "Bet not found or already resolved."
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: response,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}
		return
	}

	guild, err := guildService.GetGuildInfo(s, db, bet.GuildID, bet.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	var entries []models.BetEntry
	db.Where("bet_id = ? AND deleted_at IS NULL", bet.ID).Find(&entries)

	totalPayout := 0.0
	totalWinningPayouts := 0.0
	lostPoolAmount := 0.0
	winnerDiscordIDs := make(map[string]float64)
	for _, entry := range entries {
		var user models.User
		db.First(&user, "id = ?", entry.UserID)
		if user.ID == 0 {
			continue
		}

		if entry.Option == winningOption {
			payout := common.CalculatePayout(entry.Amount, winningOption, bet)

			unoApplied, isWinAfterUno, err := cardService.ApplyUnoReverseIfApplicable(db, user, bet.ID, true)
			if err != nil {
				common.SendError(s, i, fmt.Errorf("error checking Uno Reverse: %v", err), db)
				return
			}

			if unoApplied && !isWinAfterUno {
				user.TotalBetsLost++
				user.TotalPointsLost += float64(entry.Amount)
				db.Save(&user)
				lostPoolAmount += float64(entry.Amount)
				continue
			}

			_, _, antiAntiBetLosers, antiAntiBetApplied, err := cardService.ApplyAntiAntiBetIfApplicable(db, user, true)
			if err != nil {
				common.SendError(s, i, fmt.Errorf("error checking Anti-Anti-Bet: %v", err), db)
				return
			}
			if antiAntiBetApplied {
				if len(antiAntiBetLosers) > 0 {
					for _, loser := range antiAntiBetLosers {
						cardHolderUsername := common.GetUsernameWithDB(db, s, user.GuildID, loser.DiscordID)
						loserList += fmt.Sprintf("%s - **Lost $%.1f** (Anti-Anti-Bet!)\n", cardHolderUsername, loser.Payout)
					}
				}
			}

			consumer := func(db *gorm.DB, user models.User, cardID uint) error {
				return cardService.PlayCardFromInventory(s, db, user, cardID)
			}

			modifiedPayout, hasDoubleDown, err := cardService.ApplyDoubleDownIfAvailable(db, consumer, user, payout)
			if err != nil {
				common.SendError(s, i, fmt.Errorf("error checking Double Down: %v", err), db)
				return
			}

			payoutAfterDoubleDown := modifiedPayout

			modifiedPayout, hasGambler, err := cardService.ApplyGamblerIfAvailable(db, consumer, user, modifiedPayout, true)
			if err != nil {
				common.SendError(s, i, fmt.Errorf("error checking Gambler: %v", err), db)
				return
			}

			_, insuranceApplied, err := cardService.ApplyBetInsuranceIfApplicable(db, consumer, user, 0, true)
			if err != nil {
				common.SendError(s, i, fmt.Errorf("error checking Bet Insurance: %v", err), db)
				return
			}

			user.Points += modifiedPayout
			user.TotalBetsWon++
			user.TotalPointsWon += modifiedPayout
			db.Save(&user)
			totalPayout += modifiedPayout
			totalWinningPayouts += modifiedPayout
			winnerDiscordIDs[user.DiscordID] += modifiedPayout

			if modifiedPayout > 0 {
				username := common.GetUsernameWithDB(db, s, user.GuildID, user.DiscordID)
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
				insuranceMsg := ""
				if insuranceApplied {
					insuranceMsg = " (Bet Insurance: consumed)"
				}
				winnersList += fmt.Sprintf("%s - Won $%.1f%s%s%s\n", username, modifiedPayout, doubleDownMsg, gamblerMsg, insuranceMsg)
			}
		} else {
			unoApplied, isWinAfterUno, err := cardService.ApplyUnoReverseIfApplicable(db, user, bet.ID, false)
			if err != nil {
				common.SendError(s, i, fmt.Errorf("error checking Uno Reverse: %v", err), db)
				return
			}

			if unoApplied && isWinAfterUno {
				payout := common.CalculatePayout(entry.Amount, entry.Option, bet)

				consumer := func(db *gorm.DB, user models.User, cardID uint) error {
					return cardService.PlayCardFromInventory(s, db, user, cardID)
				}

				modifiedPayout, hasDoubleDown, err := cardService.ApplyDoubleDownIfAvailable(db, consumer, user, payout)
				if err != nil {
					common.SendError(s, i, fmt.Errorf("error checking Double Down: %v", err), db)
					return
				}

				payoutAfterDoubleDown := modifiedPayout

				modifiedPayout, hasGambler, err := cardService.ApplyGamblerIfAvailable(db, consumer, user, modifiedPayout, true)
				if err != nil {
					common.SendError(s, i, fmt.Errorf("error checking Gambler: %v", err), db)
					return
				}

				hedgeRefund, hedgeApplied, err := cardService.ApplyEmotionalHedgeIfApplicable(db, consumer, user, bet, entry.Option, float64(entry.Amount), 0)
				if err != nil {
					common.SendError(s, i, fmt.Errorf("error checking Emotional Hedge: %v", err), db)
					return
				}

				_, insuranceApplied, err := cardService.ApplyBetInsuranceIfApplicable(db, consumer, user, 0, true)
				if err != nil {
					common.SendError(s, i, fmt.Errorf("error checking Bet Insurance: %v", err), db)
					return
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
				winnerDiscordIDs[user.DiscordID] += modifiedPayout + hedgeRefund

				username := common.GetUsernameWithDB(db, s, user.GuildID, user.DiscordID)

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

				winnersList += fmt.Sprintf("%s - Won $%.1f (Uno Reverse!)%s%s%s%s\n", username, modifiedPayout, doubleDownMsg, gamblerMsg, hedgeMsg, insuranceMsg)
				continue
			}

			antiAntiBetPayout, antiAntiBetWinners, _, antiAntiBetApplied, err := cardService.ApplyAntiAntiBetIfApplicable(db, user, false)
			if err != nil {
				common.SendError(s, i, fmt.Errorf("error checking Anti-Anti-Bet: %v", err), db)
				return
			}
			if antiAntiBetApplied && antiAntiBetPayout > 0 {
				totalPayout += antiAntiBetPayout

				if len(antiAntiBetWinners) > 0 {
					for _, winner := range antiAntiBetWinners {
						username := common.GetUsernameWithDB(db, s, user.GuildID, winner.DiscordID)
						winnersList += fmt.Sprintf("%s - Won $%.1f (Anti-Anti-Bet!)\n", username, winner.Payout)
					}
				}
			}

			consumer := func(db *gorm.DB, user models.User, cardID uint) error {
				return cardService.PlayCardFromInventory(s, db, user, cardID)
			}

			jailRefund, jailApplied, err := cardService.ApplyGetOutOfJailIfApplicable(db, consumer, user, float64(entry.Amount))
			if err != nil {
				common.SendError(s, i, fmt.Errorf("error checking Get Out of Jail Free: %v", err), db)
				return
			}

			if jailApplied && jailRefund > 0 {
				user.Points += jailRefund
				db.Save(&user)
				username := common.GetUsernameWithDB(db, s, user.GuildID, user.DiscordID)
				loserList += fmt.Sprintf("%s - **Lost $%.1f** (Get Out of Jail Free: Full refund!)\n", username, float64(entry.Amount))
				continue
			}

			insuranceRefund, insuranceApplied, err := cardService.ApplyBetInsuranceIfApplicable(db, consumer, user, float64(entry.Amount), false)
			if err != nil {
				common.SendError(s, i, fmt.Errorf("error checking Bet Insurance: %v", err), db)
				return
			}

			lossAmount := float64(entry.Amount)
			modifiedLoss, hasGambler, err := cardService.ApplyGamblerIfAvailable(db, consumer, user, -lossAmount, false)
			if err != nil {
				common.SendError(s, i, fmt.Errorf("error checking Gambler: %v", err), db)
				return
			}

			actualLoss := lossAmount
			if hasGambler && modifiedLoss < -lossAmount {
				actualLoss = -modifiedLoss
			}

			user.TotalBetsLost++
			user.TotalPointsLost += actualLoss

			if insuranceApplied && insuranceRefund > 0 {
				user.Points += insuranceRefund
				lostPoolAmount -= insuranceRefund
			}

			db.Save(&user)
			lostPoolAmount += actualLoss
		}
	}

	if totalWinningPayouts > 0 {
		vampirePayout, vampireWinners, vampireApplied, err := cardService.ApplyVampireIfApplicable(db, bet.GuildID, totalWinningPayouts, winnerDiscordIDs)
		if err != nil {
			common.SendError(s, i, fmt.Errorf("error checking Vampire: %v", err), db)
			return
		}
		if vampireApplied && vampirePayout > 0 {
			totalPayout += vampirePayout
			if len(vampireWinners) > 0 {
				for _, winner := range vampireWinners {
					username := common.GetUsernameWithDB(db, s, bet.GuildID, winner.DiscordID)
					winnersList += fmt.Sprintf("%s - Won $%.1f (Vampire)\n", username, winner.Payout)
				}
			}
		}
	}

	if lostPoolAmount > 0 {
		db.Model(&models.Guild{}).Where("id = ?", guild.ID).UpdateColumn("pool", gorm.Expr("pool + ?", lostPoolAmount))
	}

	bet.Active = false
	db.Model(&bet).UpdateColumn("paid", true).UpdateColumn("active", false)

	err = UpdateParlaysOnBetResolution(s, db, bet.ID, winningOption, 0)
	if err != nil {
		fmt.Printf("Error updating parlays for bet %d: %v\n", bet.ID, err)
	}

	winningOptionName := bet.Option1
	if winningOption == 2 {
		winningOptionName = bet.Option2
	}

	winnersText := strings.TrimSpace(winnersList)
	losersText := strings.TrimSpace(loserList)
	embed := messageService.BuildBetResolutionEmbed(
		bet.Description,
		fmt.Sprintf("Winning option: **%s**", winningOptionName),
		totalPayout,
		winnersText,
		losersText,
	)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
}

func MyOpenBets(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	var bets []models.BetEntry
	userID := i.Member.User.ID

	result := db.
		Preload("Bet").
		Joins("JOIN bets ON bet_entries.bet_id = bets.id").
		Joins("JOIN users ON bet_entries.user_id = users.id").
		Where("users.discord_id = ? AND bets.paid = 0 AND bets.guild_id = ? and bet_entries.deleted_at is null", userID, i.GuildID).
		Find(&bets)
	if result.Error != nil {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Error finding active bets.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}
		return
	}

	if len(bets) == 0 {
		embed := &discordgo.MessageEmbed{
			Title:       "📊 Your Active Bets",
			Description: "You have no active bets.",
			Color:       0x5865F2,
		}
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	var fields []*discordgo.MessageEmbedField
	for idx, bet := range bets {
		var fieldValue string
		var optionName string

		if bet.Spread != nil {
			if bet.Option == 1 {
				optionName = fmt.Sprintf("Home %s", common.FormatOdds(*bet.Spread))
			} else {
				spreadVal := *bet.Spread * -1
				optionName = fmt.Sprintf("Away %s", common.FormatOdds(spreadVal))
			}
		} else {
			if bet.Option == 1 {
				optionName = bet.Bet.Option1
			} else {
				optionName = bet.Bet.Option2
			}
		}

		fieldValue = fmt.Sprintf("**%s**\n💰 Amount: %d points", optionName, bet.Amount)

		fieldName := fmt.Sprintf("%d. %s", idx+1, bet.Bet.Description)

		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fieldName,
			Value:  fieldValue,
			Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("📊 Your Active Bets (%d)", len(bets)),
		Description: "Here are your currently active bets:",
		Fields:      fields,
		Color:       0x5865F2,
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
}
