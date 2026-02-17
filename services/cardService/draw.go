package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)
func DrawCard(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	if !guild.CardDrawingEnabled {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Card drawing is currently disabled for this server.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: userID, GuildID: guildID})
	if result.Error != nil {
		common.SendError(s, i, fmt.Errorf("error fetching user: %v", result.Error), db)
		return
	}
	if result.RowsAffected == 1 {
		user.Points = guild.StartingPoints
		db.Save(&user)
	}

	username := common.GetUsernameFromUser(i.Member.User)
	common.UpdateUserUsername(db, &user, username)

	now := time.Now()

	resetPeriod := time.Duration(guild.CardDrawCooldownMinutes) * time.Minute

	shouldResetCount := false
	if user.FirstCardDrawCycle != nil {
		timeSinceFirstDraw := now.Sub(*user.FirstCardDrawCycle)
		if timeSinceFirstDraw >= resetPeriod {
			user.FirstCardDrawCycle = &now
			user.CardDrawCount = 0
			shouldResetCount = true
		}
	} else {
		user.FirstCardDrawCycle = &now
		user.CardDrawCount = 0
		shouldResetCount = true
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var drawCardCost float64
	switch user.CardDrawCount {
	case 0:
		drawCardCost = guild.CardDrawCost
	case 1:
		drawCardCost = guild.CardDrawCost * 10
	default:
		drawCardCost = guild.CardDrawCost * 50
	}

	var donorUserID uint
	var donorName string
	if drawCardCost == guild.CardDrawCost {
		donorID, err := hasGenerousDonationInInventory(db, guildID)
		if err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error checking donation inventory: %v", err), db)
			return
		}

		if donorID != 0 && donorID != user.ID {
			donorUserID = donorID
			var donor models.User
			if err := db.First(&donor, donorID).Error; err == nil {
				donorName = common.GetUsernameWithDB(db, s, guildID, donor.DiscordID)
			}

			drawCardCost = 0
		}
	}

	var chariotInventory models.UserInventory
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL",
			user.ID, guildID, cards.TheChariotCardID).
		First(&chariotInventory).Error
	if err == nil {
		if chariotInventory.TimesApplied < 3 {
			drawCardCost = 0
			chariotInventory.TimesApplied++
			if chariotInventory.TimesApplied == 3 {
				if err := tx.Delete(&chariotInventory).Error; err != nil {
					tx.Rollback()
					common.SendError(s, i, fmt.Errorf("error deleting The Chariot after 3 uses: %v", err), db)
					return
				}
			} else {
				if err := tx.Save(&chariotInventory).Error; err != nil {
					tx.Rollback()
					common.SendError(s, i, fmt.Errorf("error updating The Chariot: %v", err), db)
					return
				}
			}
		}
	} else if err != gorm.ErrRecordNotFound {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error checking The Chariot inventory: %v", err), db)
		return
	}

	var inventoryItems []models.UserInventory
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND guild_id = ? AND card_id IN (?, ?, ?)",
			user.ID, guildID,
			cards.LuckyHorseshoeCardID, cards.UnluckyCatCardID, cards.CouponCardID).
		Find(&inventoryItems).Error
	if err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error checking inventory: %v", err), db)
		return
	}

	var shoppingSpreeItems []models.UserInventory
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND guild_id = ? AND card_id = ? AND deleted_at IS NULL",
			user.ID, guildID, cards.ShoppingSpreeCardID).
		Order("created_at DESC").
		Find(&shoppingSpreeItems).Error
	if err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error checking Shopping Spree inventory: %v", err), db)
		return
	}

	inventoryMap := make(map[uint]*models.UserInventory)
	for idx := range inventoryItems {
		inventoryMap[inventoryItems[idx].CardID] = &inventoryItems[idx]
	}

	var horseshoeInventory *models.UserInventory
	var shoppingSpreeInventory *models.UserInventory
	var unluckyCatInventory *models.UserInventory
	var couponInventory *models.UserInventory

	expirationTime := now.Add(-12 * time.Hour)
	var expiredShoppingSpreeItems []*models.UserInventory
	for idx := range shoppingSpreeItems {
		item := &shoppingSpreeItems[idx]
		if item.CreatedAt.Before(expirationTime) {
			expiredShoppingSpreeItems = append(expiredShoppingSpreeItems, item)
		} else {
			if shoppingSpreeInventory == nil {
				shoppingSpreeInventory = item
				inventoryMap[cards.ShoppingSpreeCardID] = item
				drawCardCost = drawCardCost * 0.5
			}
		}
	}

	for _, expiredItem := range expiredShoppingSpreeItems {
		if err := tx.Delete(expiredItem).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error deleting expired Shopping Spree: %v", err), db)
			return
		}
	}

	if inv, ok := inventoryMap[cards.LuckyHorseshoeCardID]; ok {
		horseshoeInventory = inv
		drawCardCost = drawCardCost * 0.5
	}

	if inv, ok := inventoryMap[cards.UnluckyCatCardID]; ok {
		unluckyCatInventory = inv
		drawCardCost = drawCardCost * 2.0
	}

	if inv, ok := inventoryMap[cards.CouponCardID]; ok {
		couponInventory = inv
		drawCardCost = drawCardCost * 0.75
	}

	hasLuckyHorseshoe := horseshoeInventory != nil
	hasUnluckyCat := unluckyCatInventory != nil
	hasCoupon := couponInventory != nil

	var lockedUser models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedUser, user.ID).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error locking user: %v", err), db)
		return
	}

	if lockedUser.CardDrawTimeoutUntil != nil && now.Before(*lockedUser.CardDrawTimeoutUntil) {
		tx.Rollback()
		timeRemaining := lockedUser.CardDrawTimeoutUntil.Sub(now)
		minutesRemaining := int(timeRemaining.Minutes())
		secondsRemaining := int(timeRemaining.Seconds()) % 60
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("You are timed out from drawing cards. Time remaining: %d minutes and %d seconds.", minutesRemaining, secondsRemaining),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	if lockedUser.CardDrawTimeoutUntil != nil && now.After(*lockedUser.CardDrawTimeoutUntil) {
		lockedUser.CardDrawTimeoutUntil = nil
	}

	if lockedUser.Points < drawCardCost {
		tx.Rollback()
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("You need at least %.0f points to draw a card. You have %.1f points.", drawCardCost, lockedUser.Points),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	if hasLuckyHorseshoe {
		if err := tx.Delete(horseshoeInventory).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error consuming Lucky Horseshoe: %v", err), db)
			return
		}
	}

	if hasUnluckyCat {
		if err := tx.Delete(unluckyCatInventory).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error consuming Unlucky Cat: %v", err), db)
			return
		}
	}

	if hasCoupon {
		if err := tx.Delete(couponInventory).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error consuming Coupon: %v", err), db)
			return
		}
	}

	user = lockedUser

	if shouldResetCount {
		user.CardDrawCount = 0
		user.FirstCardDrawCycle = &now
	}

	if donorUserID != 0 {
		var donorUser models.User
		if err := tx.First(&donorUser, donorUserID).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error fetching donor user: %v", err), db)
			return
		}

		if err := PlayCardFromInventory(s, tx, donorUser, cards.GenerousDonationCardID); err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error consuming Generous Donation: %v", err), db)
			return
		}
	}

	var lockedGuild models.Guild
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedGuild, guild.ID).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error locking guild: %v", err), db)
		return
	}

	*guild = lockedGuild

	user.Points -= drawCardCost
	guild.Pool += drawCardCost

	if guild.PoolDrainUntil != nil {
		if now.After(*guild.PoolDrainUntil) {
			guild.PoolDrainUntil = nil
		} else {
			poolDrainAmount := 100.0
			if guild.Pool >= poolDrainAmount {
				guild.Pool -= poolDrainAmount
			} else {
				guild.Pool = 0
			}
		}
	}

	user.CardDrawCount++

	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return
	}

	if err := tx.Save(&guild).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return
	}

	guild.TotalCardDraws++
	if err := tx.Save(&guild).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return
	}

	_, distanceFromTop5, err := GetUserRankFromTop5(tx, user.ID, guildID)
	if err != nil {
		distanceFromTop5 = 0
	}
	rarityMultiplier := calculateRarityMultiplier(distanceFromTop5)

	hasSubscription := guild.SubscribedTeam != nil && *guild.SubscribedTeam != ""

	var card *models.Card
	needsMythic := guild.TotalCardDraws >= 1000 && (guild.LastMythicDrawAt == 0 || (guild.TotalCardDraws-guild.LastMythicDrawAt) >= 1000)
	needsEpic := guild.TotalCardDraws >= 100 && (guild.LastEpicDrawAt == 0 || (guild.TotalCardDraws-guild.LastEpicDrawAt) >= 100)

	if needsMythic {
		card = PickCardByRarity(hasSubscription, "Mythic")
		if card == nil {
			card = PickRandomCard(hasSubscription, rarityMultiplier, guild)
		}
	} else if needsEpic {
		card = PickCardByRarity(hasSubscription, "Epic")
		if card == nil {
			card = PickRandomCard(hasSubscription, rarityMultiplier, guild)
		}
	} else {
		card = PickRandomCard(hasSubscription, rarityMultiplier, guild)
	}

	if card == nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("no cards available"), db)
		return
	}

	if card.CardRarity.Name == "Mythic" {
		guild.LastMythicDrawAt = guild.TotalCardDraws
	} else if card.CardRarity.Name == "Epic" {
		guild.LastEpicDrawAt = guild.TotalCardDraws
	}
	if err := tx.Save(&guild).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return
	}

	if err := processRoyaltyPayment(tx, card, cards.RoyaltyGuildID); err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error processing royalty payment: %v", err), db)
		return
	}

	if card.AddToInventory || card.UserPlayable {
		if err := addCardToInventory(tx, user.ID, guildID, card.ID); err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error adding card to inventory: %v", err), db)
			return
		}
	}

	cardResult, err := card.Handler(s, tx, userID, guildID)
	if err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error executing card effect: %v", err), db)
		return
	}

	// Reload user to capture any changes made by the card handler
	if err := tx.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error reloading user after card effect: %v", err), db)
		return
	}

	if err := processTagCards(tx, guildID); err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error processing tag cards: %v", err), db)
		return
	}

	if cardResult.RequiresSelection {
		if cardResult.SelectionType == "user" {
			if card.ID == cards.HostileTakeoverCardID || card.ID == cards.JusticeCardID {
				ShowFilteredUserSelectMenu(s, i, card.ID, card.Name, card.Description, userID, guildID, tx, 500.0)
			} else {
				ShowUserSelectMenu(s, i, card.ID, card.Name, card.Description, userID, guildID, db)
			}

			if err := tx.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}

			user.Points += cardResult.PointsDelta
			if user.Points < 0 {
				user.Points = 0
			}
			guild.Pool += cardResult.PoolDelta

			if err := tx.Save(&user).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}
			if err := tx.Save(&guild).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}

			tx.Commit()
			return
		} else if cardResult.SelectionType == "bet" {
			ShowBetSelectMenu(s, i, card.ID, card.Name, card.Description, userID, guildID, db)

			user.Points += cardResult.PointsDelta
			if user.Points < 0 {
				user.Points = 0
			}
			guild.Pool += cardResult.PoolDelta

			if err := tx.Save(&user).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}
			if err := tx.Save(&guild).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}

			tx.Commit()
			return
		}
	}

	if len(card.Options) > 0 {
		ShowCardOptionsMenu(s, i, card.ID, card.Name, card.Description, userID, guildID, db, card.Options)

		user.Points += cardResult.PointsDelta
		if user.Points < 0 {
			user.Points = 0
		}
		guild.Pool += cardResult.PoolDelta

		if err := tx.Save(&user).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, err, db)
			return
		}
		if err := tx.Save(&guild).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, err, db)
			return
		}

		tx.Commit()
		return
	}

	if cardResult.PointsDelta < 0 {
		hasMoon, err := hasMoonInInventory(tx, user.ID, guildID)
		if err != nil {
			tx.Rollback()
			common.SendError(s, i, err, db)
			return
		}
		if hasMoon {
			if err := PlayCardFromInventory(s, tx, user, cards.TheMoonCardID); err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}
			randomUserID, err := getRandomUserForMoonFromCards(tx, guildID, []uint{user.ID})
			if err != nil {
				hasShield, err := hasShieldInInventory(tx, user.ID, guildID)
				if err != nil {
					tx.Rollback()
					common.SendError(s, i, err, db)
					return
				}
				if hasShield {
					if err := PlayCardFromInventory(s, tx, user, cards.ShieldCardID); err != nil {
						tx.Rollback()
						common.SendError(s, i, err, db)
						return
					}
					cardResult.PointsDelta = 0
					if cardResult.Message == "" {
						cardResult.Message = "Your Moon illusion tried to redirect, but no eligible users found. Shield blocked the hit!"
					} else {
						cardResult.Message += " (Your Moon illusion tried to redirect, but no eligible users found. Shield blocked the hit!)"
					}
				} else {
					if cardResult.Message == "" {
						cardResult.Message = "Your Moon illusion tried to redirect, but no eligible users found."
					} else {
						cardResult.Message += " (Your Moon illusion tried to redirect, but no eligible users found.)"
					}
				}
			} else {
				var randomUser models.User
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("discord_id = ? AND guild_id = ?", randomUserID, guildID).
					First(&randomUser).Error; err != nil {
					tx.Rollback()
					common.SendError(s, i, err, db)
					return
				}

				redirectedLoss := -cardResult.PointsDelta
				if randomUser.Points < redirectedLoss {
					redirectedLoss = randomUser.Points
				}

				randomMention := "<@" + randomUserID + ">"
				cardResult.PointsDelta = 0
				cardResult.TargetUserID = &randomUserID
				cardResult.TargetPointsDelta = -redirectedLoss
				if cardResult.Message == "" {
					cardResult.Message = fmt.Sprintf("Your Moon illusion redirected the hit! %s lost %.0f points instead!", randomMention, redirectedLoss)
				} else {
					cardResult.Message += fmt.Sprintf(" (Your Moon illusion redirected the hit! %s lost %.0f points instead!)", randomMention, redirectedLoss)
				}
			}
		} else {
			hasShield, err := hasShieldInInventory(tx, user.ID, guildID)
			if err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return
			}
			if hasShield {
				if err := PlayCardFromInventory(s, tx, user, cards.ShieldCardID); err != nil {
					tx.Rollback()
					common.SendError(s, i, err, db)
					return
				}

				cardResult.PointsDelta = 0
				if cardResult.Message == "" {
					cardResult.Message = "Your Shield blocked the hit!"
				} else {
					cardResult.Message += " (Your Shield blocked the hit!)"
				}
			}
		}
	}

	user.Points += cardResult.PointsDelta
	if user.Points < 0 {
		user.Points = 0
	}
	guild.Pool += cardResult.PoolDelta

	var targetUsername string
	if cardResult.TargetUserID != nil {
		var targetUser models.User
		if err := tx.Where("discord_id = ? AND guild_id = ?", *cardResult.TargetUserID, guildID).First(&targetUser).Error; err == nil {
			if cardResult.TargetPointsDelta < 0 {
				targetMention := "<@" + *cardResult.TargetUserID + ">"
				hasMoon, err := hasMoonInInventory(tx, targetUser.ID, guildID)
				if err != nil {
					tx.Rollback()
					common.SendError(s, i, err, db)
					return
				}
				if hasMoon {
					if err := PlayCardFromInventory(s, tx, targetUser, cards.TheMoonCardID); err != nil {
						tx.Rollback()
						common.SendError(s, i, err, db)
						return
					}
					randomUserID, err := getRandomUserForMoonFromCards(tx, guildID, []uint{targetUser.ID, user.ID})
					if err != nil {
						hasShield, err := hasShieldInInventory(tx, targetUser.ID, guildID)
						if err != nil {
							tx.Rollback()
							common.SendError(s, i, err, db)
							return
						}
						if hasShield {
							if err := PlayCardFromInventory(s, tx, targetUser, cards.ShieldCardID); err != nil {
								tx.Rollback()
								common.SendError(s, i, err, db)
								return
							}
							cardResult.TargetPointsDelta = 0
							if cardResult.Message == "" {
								cardResult.Message = fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found. Shield blocked the hit!", targetMention)
							} else {
								cardResult.Message += fmt.Sprintf(" (%s's Moon illusion tried to redirect, but no eligible users found. Shield blocked the hit!)", targetMention)
							}
						} else {
							if cardResult.Message == "" {
								cardResult.Message = fmt.Sprintf("%s's Moon illusion tried to redirect, but no eligible users found.", targetMention)
							} else {
								cardResult.Message += fmt.Sprintf(" (%s's Moon illusion tried to redirect, but no eligible users found.)", targetMention)
							}
						}
					} else {
						var randomUser models.User
						if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
							Where("discord_id = ? AND guild_id = ?", randomUserID, guildID).
							First(&randomUser).Error; err != nil {
							tx.Rollback()
							common.SendError(s, i, err, db)
							return
						}

						redirectedLoss := -cardResult.TargetPointsDelta
						if randomUser.Points < redirectedLoss {
							redirectedLoss = randomUser.Points
						}

						randomMention := "<@" + randomUserID + ">"
						cardResult.TargetUserID = &randomUserID
						cardResult.TargetPointsDelta = -redirectedLoss
						if cardResult.Message == "" {
							cardResult.Message = fmt.Sprintf("%s's Moon illusion redirected the hit! %s lost %.0f points instead!", targetMention, randomMention, redirectedLoss)
						} else {
							cardResult.Message += fmt.Sprintf(" (%s's Moon illusion redirected the hit! %s lost %.0f points instead!)", targetMention, randomMention, redirectedLoss)
						}
					}
				} else {
					hasShield, err := hasShieldInInventory(tx, targetUser.ID, guildID)
					if err != nil {
						tx.Rollback()
						common.SendError(s, i, err, db)
						return
					}
					if hasShield {
						if err := PlayCardFromInventory(s, tx, targetUser, cards.ShieldCardID); err != nil {
							tx.Rollback()
							common.SendError(s, i, err, db)
							return
						}

						cardResult.TargetPointsDelta = 0
						if cardResult.Message == "" {
							cardResult.Message = fmt.Sprintf("%s's Shield blocked the hit!", targetMention)
						} else {
							cardResult.Message += fmt.Sprintf(" (%s's Shield blocked the hit!)", targetMention)
						}
					}
				}
			}

			var userToUpdate models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("discord_id = ? AND guild_id = ?", *cardResult.TargetUserID, guildID).
				First(&userToUpdate).Error; err == nil {
				userToUpdate.Points += cardResult.TargetPointsDelta
				if userToUpdate.Points < 0 {
					userToUpdate.Points = 0
				}
				if err := tx.Save(&userToUpdate).Error; err != nil {
					tx.Rollback()
					common.SendError(s, i, err, db)
					return
				}
			}
			targetUsername = common.GetUsernameWithDB(db, s, guildID, *cardResult.TargetUserID)
		}
	}

	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return
	}
	if err := tx.Save(&guild).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return
	}

	tx.Commit()

	username = common.GetUsernameWithDB(db, s, guildID, user.DiscordID)
	embed := buildCardEmbed(card, cardResult, user, username, targetUsername, guild.Pool, drawCardCost)

	if donorUserID != 0 && donorName != "" {
		if embed.Footer == nil {
			embed.Footer = &discordgo.MessageEmbedFooter{}
		}
		originalText := embed.Footer.Text
		embed.Footer.Text = fmt.Sprintf("%s | Paid for by generous donation from %s!", originalText, donorName)
	}

	var content string
	if card.ID == cards.RickRollCardID {
		content = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Embeds:  []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	if card.ID == cards.SpyKids3DCardID {
		var currentGuild models.Guild
		if err := db.Where("guild_id = ?", guildID).First(&currentGuild).Error; err != nil {
			return
		}

		epicDrawsRemaining := 0
		if currentGuild.LastEpicDrawAt == 0 {
			if currentGuild.TotalCardDraws < 100 {
				epicDrawsRemaining = 100 - currentGuild.TotalCardDraws
			} else {
				epicDrawsRemaining = 0
			}
		} else {
			drawsSinceLastEpic := currentGuild.TotalCardDraws - currentGuild.LastEpicDrawAt
			if drawsSinceLastEpic >= 100 {
				epicDrawsRemaining = 0
			} else {
				epicDrawsRemaining = 100 - drawsSinceLastEpic
			}
		}

		mythicDrawsRemaining := 0
		if currentGuild.LastMythicDrawAt == 0 {
			if currentGuild.TotalCardDraws < 1000 {
				mythicDrawsRemaining = 1000 - currentGuild.TotalCardDraws
			} else {
				mythicDrawsRemaining = 0
			}
		} else {
			drawsSinceLastMythic := currentGuild.TotalCardDraws - currentGuild.LastMythicDrawAt
			if drawsSinceLastMythic >= 1000 {
				mythicDrawsRemaining = 0
			} else {
				mythicDrawsRemaining = 1000 - drawsSinceLastMythic
			}
		}

		epicText := fmt.Sprintf("%d cards", epicDrawsRemaining)
		if epicDrawsRemaining == 0 {
			epicText = "Next draw!"
		}

		mythicText := fmt.Sprintf("%d cards", mythicDrawsRemaining)
		if mythicDrawsRemaining == 0 {
			mythicText = "Next draw!"
		}

		infoEmbed := &discordgo.MessageEmbed{
			Title:       "🎴 Guaranteed Card Information",
			Description: "Here's when the next guaranteed cards will be drawn:",
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Next Guaranteed Epic",
					Value:  epicText,
					Inline: true,
				},
				{
					Name:   "Next Guaranteed Mythic",
					Value:  mythicText,
					Inline: true,
				},
			},
			Color: 0x95A5A6,
		}

		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{infoEmbed},
			Flags:  discordgo.MessageFlagsEphemeral,
		})
		if err != nil {
			fmt.Printf("Error sending Spy Kids 3D follow-up message: %v\n", err)
		}
	}
}
