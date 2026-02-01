package cardService

import (
	"fmt"
	"math"
	"math/rand"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CardConsumer func(db *gorm.DB, user models.User, cardID uint) error

const (
	selectorTTL             = 3600
	selectorCleanupInterval = 900
)

var (
	usedCardSelectors = make(map[string]int64)
	usedSelectorsMu   sync.RWMutex
)

func init() {
	go startSelectorCleanup()
}

func IsSelectorUsed(customID string) bool {
	usedSelectorsMu.RLock()
	defer usedSelectorsMu.RUnlock()
	timestamp, exists := usedCardSelectors[customID]
	if !exists {
		return false
	}
	now := time.Now().Unix()
	return (now - timestamp) < selectorTTL
}

func MarkSelectorUsed(customID string) {
	usedSelectorsMu.Lock()
	defer usedSelectorsMu.Unlock()
	usedCardSelectors[customID] = time.Now().Unix()
}

func TryMarkSelectorUsed(customID string) bool {
	usedSelectorsMu.Lock()
	defer usedSelectorsMu.Unlock()

	now := time.Now().Unix()
	timestamp, exists := usedCardSelectors[customID]

	if exists && (now-timestamp) < selectorTTL {
		return false
	}

	usedCardSelectors[customID] = now
	return true
}

func UnmarkSelectorUsed(customID string) {
	usedSelectorsMu.Lock()
	defer usedSelectorsMu.Unlock()
	delete(usedCardSelectors, customID)
}

func startSelectorCleanup() {
	ticker := time.NewTicker(time.Duration(selectorCleanupInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().Unix()
		usedSelectorsMu.Lock()
		for key, timestamp := range usedCardSelectors {
			if (now - timestamp) >= selectorTTL {
				delete(usedCardSelectors, key)
			}
		}
		usedSelectorsMu.Unlock()
	}
}

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
		drawCardCost = guild.CardDrawCost * 100
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

	// Check timeout after locking user to ensure we have the most up-to-date value
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
			card = PickRandomCard(hasSubscription, rarityMultiplier)
		}
	} else if needsEpic {
		card = PickCardByRarity(hasSubscription, "Epic")
		if card == nil {
			card = PickRandomCard(hasSubscription, rarityMultiplier)
		}
	} else {
		card = PickRandomCard(hasSubscription, rarityMultiplier)
	}

	if card == nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("no cards available"), db)
		return
	}

	// Update tracking for Epic and Mythic cards
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
			randomUserID, err := getRandomUserForMoonFromCards(tx, guildID, []uint{user.ID})
			if err != nil {
				// Fall back to shield if no eligible users
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

				randomUser.Points -= redirectedLoss
				if err := tx.Save(&randomUser).Error; err != nil {
					tx.Rollback()
					common.SendError(s, i, err, db)
					return
				}

				if err := PlayCardFromInventory(s, tx, user, cards.TheMoonCardID); err != nil {
					tx.Rollback()
					common.SendError(s, i, err, db)
					return
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
					randomUserID, err := getRandomUserForMoonFromCards(tx, guildID, []uint{targetUser.ID, user.ID})
					if err != nil {
						// Fall back to shield if no eligible users
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

						randomUser.Points -= redirectedLoss
						if err := tx.Save(&randomUser).Error; err != nil {
							tx.Rollback()
							common.SendError(s, i, err, db)
							return
						}

						if err := PlayCardFromInventory(s, tx, targetUser, cards.TheMoonCardID); err != nil {
							tx.Rollback()
							common.SendError(s, i, err, db)
							return
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

			targetUser.Points += cardResult.TargetPointsDelta
			if targetUser.Points < 0 {
				targetUser.Points = 0
			}
			tx.Save(&targetUser)
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
			// Log error but don't fail the card draw
			fmt.Printf("Error sending Spy Kids 3D follow-up message: %v\n", err)
		}
	}
}

func ShowUserSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, cardID uint, cardName string, cardDescription string, userID string, guildID string, db *gorm.DB) {
	minValues := 1
	selectorID := i.Interaction.ID
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 <@%s> drew **%s**!\n%s\n\nSelect a user to target:", userID, cardName, cardDescription),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.UserSelectMenu,
							CustomID:    fmt.Sprintf("card_%d_select_%s_%s_%s", cardID, userID, guildID, selectorID),
							Placeholder: "Choose a user...",
							MinValues:   &minValues,
							MaxValues:   1,
						},
					},
				},
			},
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
	}
}

func ShowFilteredUserSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, cardID uint, cardName string, cardDescription string, userID string, guildID string, db *gorm.DB, maxPointDifference float64) {
	var drawer models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&drawer).Error; err != nil {
		common.SendError(s, i, err, db)
		return
	}

	var allUsers []models.User
	if err := db.Where("guild_id = ? AND discord_id != ?", guildID, userID).Find(&allUsers).Error; err != nil {
		common.SendError(s, i, err, db)
		return
	}

	var eligibleUsers []models.User
	for _, u := range allUsers {
		pointDiff := math.Abs(drawer.Points - u.Points)
		if pointDiff <= maxPointDifference {
			eligibleUsers = append(eligibleUsers, u)
		}
	}

	if len(eligibleUsers) == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("🎴 <@%s> drew **%s**!\n%s\n\nNo users found within %.0f points of you (you have %.1f points). Hostile Takeover fizzles out.", userID, cardName, cardDescription, maxPointDifference, drawer.Points),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	var selectOptions []discordgo.SelectMenuOption
	for _, u := range eligibleUsers {
		username := u.Username
		displayName := ""
		if username == nil || *username == "" {
			displayName = fmt.Sprintf("User %s", u.DiscordID)
		} else {
			displayName = *username
		}

		if len(displayName) > 100 {
			displayName = displayName[:97] + "..."
		}

		description := fmt.Sprintf("%.1f points", u.Points)
		if len(description) > 100 {
			description = description[:97] + "..."
		}

		value := u.DiscordID
		if len(value) == 0 {
			continue
		}
		if len(value) > 25 {
			value = value[:25]
		}

		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label:       displayName,
			Value:       value,
			Description: description,
			Emoji:       nil,
			Default:     false,
		})
	}

	selectorID := i.Interaction.ID
	minValues := 1
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 <@%s> drew **%s**!\n%s\n\nSelect a user within %.0f points of you (you have %.1f points):", userID, cardName, cardDescription, maxPointDifference, drawer.Points),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("card_%d_select_%s_%s_%s", cardID, userID, guildID, selectorID),
							Placeholder: "Choose a user...",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     selectOptions,
						},
					},
				},
			},
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
	}
}

func ShowBetSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, cardID uint, cardName string, cardDescription string, userID string, guildID string, db *gorm.DB) {
	var results []struct {
		BetID       uint
		Description string
		Option      int
		Option1     string
		Option2     string
	}

	err := db.Table("bet_entries").
		Select("bets.id as bet_id, bets.description, bet_entries.option, bets.option1, bets.option2").
		Joins("JOIN bets ON bets.id = bet_entries.bet_id").
		Where("bet_entries.user_id = (SELECT id FROM users WHERE discord_id = ? AND guild_id = ?) AND bets.paid = ?", userID, guildID, false).
		Limit(25).
		Scan(&results).Error

	if err != nil {
		common.SendError(s, i, fmt.Errorf("error fetching bets: %v", err), db)
		return
	}

	if len(results) == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("🎴 <@%s> drew **%s**!\n%s\n\nYou have no active bets to use this card on! The card fizzles out.", userID, cardName, cardDescription),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	options := []discordgo.SelectMenuOption{}
	for _, res := range results {
		pickedTeam := res.Option1
		if res.Option == 2 {
			pickedTeam = res.Option2
		}

		label := fmt.Sprintf("%s (Pick: %s)", res.Description, pickedTeam)
		if len(label) > 100 {
			label = label[:97] + "..."
		}

		value := fmt.Sprintf("%d", res.BetID)
		if len(value) == 0 {
			continue
		}
		if len(value) > 25 {
			value = value[:25]
		}

		options = append(options, discordgo.SelectMenuOption{
			Label: label,
			Value: value,
		})
	}

	selectorID := i.Interaction.ID
	minValues := 1
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 <@%s> drew **%s**!\n%s\n\nSelect an active bet to target:", userID, cardName, cardDescription),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("card_%d_selectbet_%s_%s_%s", cardID, userID, guildID, selectorID),
							Placeholder: "Choose a bet...",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     options,
						},
					},
				},
			},
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
	}
}

func ShowCardOptionsMenu(s *discordgo.Session, i *discordgo.InteractionCreate, cardID uint, cardName string, cardDescription string, userID string, guildID string, db *gorm.DB, options []models.CardOption) {
	var selectOptions []discordgo.SelectMenuOption
	for _, opt := range options {
		label := opt.Name
		description := opt.Description

		if len(label) > 100 {
			label = label[:97] + "..."
		}
		if len(description) > 100 {
			description = description[:97] + "..."
		}

		value := fmt.Sprintf("%d", opt.ID)
		if len(value) == 0 {
			continue
		}
		if len(value) > 25 {
			value = value[:25]
		}

		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label:       label,
			Value:       value,
			Description: description,
			Emoji:       nil,
			Default:     false,
		})
	}

	if len(selectOptions) == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("🎴 <@%s> drew **%s**!\n%s\n\nNo valid options available for this card.", userID, cardName, cardDescription),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	var optionsList strings.Builder
	for i, opt := range options {
		if i > 0 {
			optionsList.WriteString("\n")
		}
		optionsList.WriteString(fmt.Sprintf("**%s**: %s", opt.Name, opt.Description))
	}

	selectorID := i.Interaction.ID
	minValues := 1
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 <@%s> drew **%s**!\n%s\n\n%s\n\nSelect an option:", userID, cardName, cardDescription, optionsList.String()),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("card_%d_option_%s_%s_%s", cardID, userID, guildID, selectorID),
							Placeholder: "Choose an option...",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     selectOptions,
						},
					},
				},
			},
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
	}
}

func ParseHexColor(colorStr string) int {
	if colorStr == "" {
		return 0x95A5A6
	}
	// Normalize to lowercase for case-insensitive prefix matching
	colorStrLower := strings.ToLower(colorStr)
	if len(colorStrLower) > 2 && colorStrLower[0:2] == "0x" {
		colorStr = colorStr[2:]
	}
	var color int
	_, err := fmt.Sscanf(colorStr, "%x", &color)
	if err != nil {
		return 0x95A5A6
	}
	return color
}

func buildCardEmbed(card *models.Card, result *models.CardResult, user models.User, username string, targetUsername string, poolBalance float64, drawCardCost float64) *discordgo.MessageEmbed {
	var color int
	if card.CardRarity.ID != 0 {
		color = ParseHexColor(card.CardRarity.Color)
	} else {
		color = 0x95A5A6 // Default to Common color
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎴 %s Drew: %s", username, card.Name),
		Description: card.Description,
		Color:       color,
		Fields:      []*discordgo.MessageEmbedField{},
	}

	rarityName := "Common"
	if card.CardRarity.ID != 0 {
		rarityName = card.CardRarity.Name
	}
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Rarity",
		Value:  rarityName,
		Inline: true,
	})

	if result.Message != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Effect",
			Value:  result.Message,
			Inline: false,
		})
	}

	if result.PointsDelta != 0 {
		sign := "+"
		if result.PointsDelta < 0 {
			sign = ""
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Points Change",
			Value:  fmt.Sprintf("<@%s>: %s%.1f points (Total: %.1f)", user.DiscordID, sign, result.PointsDelta, user.Points),
			Inline: true,
		})
	}

	if result.TargetUserID != nil && result.TargetPointsDelta != 0 {
		sign := "+"
		if result.TargetPointsDelta < 0 {
			sign = ""
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Target Change",
			Value:  fmt.Sprintf("<@%s>: %s%.1f points", *result.TargetUserID, sign, result.TargetPointsDelta),
			Inline: true,
		})
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Pool Balance",
		Value:  fmt.Sprintf("%.1f points", poolBalance),
		Inline: true,
	})

	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Cost: -%.0f points | Added %.0f to pool", drawCardCost, drawCardCost),
	}

	return embed
}

func hasLuckyHorseshoeInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.LuckyHorseshoeCardID).
		Count(&count).Error
	return count > 0, err
}

func hasUnluckyCatInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.UnluckyCatCardID).
		Count(&count).Error
	return count > 0, err
}

func hasShieldInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.ShieldCardID).
		Count(&count).Error
	return count > 0, err
}

func hasMoonInInventory(db *gorm.DB, userID uint, guildID string) (bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", userID, guildID, cards.TheMoonCardID).
		Count(&count).Error
	return count > 0, err
}

// getRandomUserForMoonFromCards is a wrapper to access GetRandomUserForMoon from cards package
func getRandomUserForMoonFromCards(db *gorm.DB, guildID string, excludeUserIDs []uint) (string, error) {
	return cards.GetRandomUserForMoon(db, guildID, excludeUserIDs)
}

func hasGenerousDonationInInventory(db *gorm.DB, guildID string) (uint, error) {
	var inventory models.UserInventory
	err := db.Model(&models.UserInventory{}).
		Where("guild_id = ? AND card_id = ?", guildID, cards.GenerousDonationCardID).
		Limit(1).
		First(&inventory).Error

	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return inventory.UserID, nil
}

func ApplyUnoReverseIfApplicable(db *gorm.DB, user models.User, betID uint, originalIsWin bool) (bool, bool, error) {
	var inventory models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ? AND card_id = ? AND target_bet_id = ?", user.ID, user.GuildID, cards.UnoReverseCardID, betID).
		First(&inventory).Error

	if err == gorm.ErrRecordNotFound {
		return false, originalIsWin, nil
	}
	if err != nil {
		return false, originalIsWin, err
	}

	if err := db.Delete(&inventory).Error; err != nil {
		return false, originalIsWin, err
	}

	return true, !originalIsWin, nil
}

type AntiAntiBetWinner struct {
	DiscordID string
	Payout    float64
}

func ApplyAntiAntiBetIfApplicable(db *gorm.DB, bettorUser models.User, isWin bool) (totalPayout float64, winners []AntiAntiBetWinner, losers []AntiAntiBetWinner, applied bool, err error) {
	var userCards []models.UserInventory
	err = db.Where("guild_id = ? AND card_id = ? AND target_user_id = ?", bettorUser.GuildID, cards.AntiAntiBetCardID, bettorUser.DiscordID).
		Find(&userCards).Error

	if err != nil {
		return 0, nil, nil, false, err
	}

	if len(userCards) == 0 {
		return 0, nil, nil, false, nil
	}

	totalPayout = 0.0
	applied = true
	winners = []AntiAntiBetWinner{}
	losers = []AntiAntiBetWinner{}

	for _, card := range userCards {
		if isWin {
			var cardHolder models.User
			if err := db.First(&cardHolder, card.UserID).Error; err != nil {
				db.Delete(&card)
				continue
			}

			losers = append(losers, AntiAntiBetWinner{
				DiscordID: cardHolder.DiscordID,
				Payout:    card.BetAmount,
			})

			if err := db.Delete(&card).Error; err != nil {
				return totalPayout, winners, losers, applied, err
			}
		} else {
			payout := common.CalculateSimplePayout(card.BetAmount)

			var cardHolder models.User
			if err := db.First(&cardHolder, card.UserID).Error; err != nil {
				db.Delete(&card)
				continue
			}

			cardHolder.Points += payout
			if err := db.Model(&cardHolder).UpdateColumn("points", gorm.Expr("points + ?", payout)).Error; err != nil {
				return totalPayout, winners, losers, applied, err
			}

			totalPayout += payout
			winners = append(winners, AntiAntiBetWinner{
				DiscordID: cardHolder.DiscordID,
				Payout:    payout,
			})

			if err := db.Delete(&card).Error; err != nil {
				return totalPayout, winners, losers, applied, err
			}
		}
	}

	return totalPayout, winners, losers, applied, nil
}

type VampireWinner struct {
	DiscordID string
	Payout    float64
}

func ApplyVampireIfApplicable(db *gorm.DB, guildID string, totalWinningPayouts float64, winnerDiscordIDs map[string]float64) (totalVampirePayout float64, winners []VampireWinner, applied bool, err error) {
	twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)
	var vampireCards []models.UserInventory
	err = db.Where("guild_id = ? AND card_id = ? AND created_at >= ?", guildID, cards.VampireCardID, twentyFourHoursAgo).
		Find(&vampireCards).Error

	if err != nil {
		return 0, nil, false, err
	}

	if len(vampireCards) == 0 {
		return 0, nil, false, nil
	}

	totalVampirePayout = 0.0
	applied = true
	winners = []VampireWinner{}

	for _, card := range vampireCards {
		var vampireHolder models.User
		if err := db.First(&vampireHolder, card.UserID).Error; err != nil {
			continue
		}

		vampireHolderWinnings := 0.0
		if winnerDiscordIDs != nil {
			vampireHolderWinnings = winnerDiscordIDs[vampireHolder.DiscordID]
		}
		totalOtherWinnings := totalWinningPayouts - vampireHolderWinnings

		if totalOtherWinnings < 0 {
			continue
		}
		if totalWinningPayouts > 0 && totalOtherWinnings == 0 {
			continue
		}

		vampirePayout := totalOtherWinnings * 0.05

		if vampirePayout > 500.0 {
			vampirePayout = 500.0
		}

		if err := db.Model(&vampireHolder).UpdateColumn("points", gorm.Expr("points + ?", vampirePayout)).Error; err != nil {
			return totalVampirePayout, winners, applied, err
		}

		totalVampirePayout += vampirePayout
		winners = append(winners, VampireWinner{
			DiscordID: vampireHolder.DiscordID,
			Payout:    vampirePayout,
		})
	}

	return totalVampirePayout, winners, applied, nil
}

type LoversWinner struct {
	DiscordID string
	Payout    float64
}

type DevilDiverted struct {
	DiscordID string
	Diverted  float64
}

func ApplyTheDevilIfApplicable(db *gorm.DB, guildID string, winnerDiscordIDs map[string]float64) (totalDiverted float64, diverted []DevilDiverted, applied bool, err error) {
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour)
	var devilCards []models.UserInventory
	err = db.Where("guild_id = ? AND card_id = ? AND created_at >= ? AND deleted_at IS NULL", guildID, cards.TheDevilCardID, sevenDaysAgo).
		Find(&devilCards).Error

	if err != nil {
		return 0, nil, false, err
	}

	if len(devilCards) == 0 {
		return 0, nil, false, nil
	}

	devilCardHolders := make(map[string]uint)
	for _, card := range devilCards {
		var user models.User
		if err := db.First(&user, card.UserID).Error; err != nil {
			continue
		}
		devilCardHolders[user.DiscordID] = user.ID
	}

	if len(devilCardHolders) == 0 {
		return 0, nil, false, nil
	}

	totalDiverted = 0.0
	applied = true
	diverted = []DevilDiverted{}

	var guild models.Guild
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return 0, nil, false, err
	}

	for discordID, winnings := range winnerDiscordIDs {
		userID, hasDevilCard := devilCardHolders[discordID]
		if !hasDevilCard || winnings <= 0 {
			continue
		}

		divertedAmount := winnings * 0.20
		totalDiverted += divertedAmount
		winnerDiscordIDs[discordID] = winnings - divertedAmount

		if err := db.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("points", gorm.Expr("points - ?", divertedAmount)).Error; err != nil {
			return totalDiverted, diverted, applied, err
		}

		diverted = append(diverted, DevilDiverted{
			DiscordID: discordID,
			Diverted:  divertedAmount,
		})
	}

	if totalDiverted > 0 {
		if err := db.Model(&guild).UpdateColumn("pool", gorm.Expr("pool + ?", totalDiverted)).Error; err != nil {
			return totalDiverted, diverted, applied, err
		}
	}

	return totalDiverted, diverted, applied, nil
}

func ApplyTheLoversIfApplicable(db *gorm.DB, guildID string, winnerDiscordIDs map[string]float64) (totalLoversPayout float64, winners []LoversWinner, applied bool, err error) {
	twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)
	var loversCards []models.UserInventory
	err = db.Where("guild_id = ? AND card_id = ? AND created_at >= ? AND target_user_id IS NOT NULL", guildID, cards.TheLoversCardID, twentyFourHoursAgo).
		Find(&loversCards).Error

	if err != nil {
		return 0, nil, false, err
	}

	if len(loversCards) == 0 {
		return 0, nil, false, nil
	}

	totalLoversPayout = 0.0
	applied = true
	winners = []LoversWinner{}

	for _, card := range loversCards {
		if card.TargetUserID == nil {
			continue
		}

		targetUserID := *card.TargetUserID
		targetWinnings, targetWon := winnerDiscordIDs[targetUserID]

		if !targetWon || targetWinnings <= 0 {
			continue
		}

		var loversHolder models.User
		if err := db.First(&loversHolder, card.UserID).Error; err != nil {
			continue
		}

		loversPayout := targetWinnings * 0.25

		if err := db.Model(&loversHolder).UpdateColumn("points", gorm.Expr("points + ?", loversPayout)).Error; err != nil {
			return totalLoversPayout, winners, applied, err
		}

		totalLoversPayout += loversPayout
		winners = append(winners, LoversWinner{
			DiscordID: loversHolder.DiscordID,
			Payout:    loversPayout,
		})
	}

	return totalLoversPayout, winners, applied, nil
}

func ApplyDoubleDownIfAvailable(db *gorm.DB, consumer CardConsumer, user models.User, originalPayout float64) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.DoubleDownCardID).
		Count(&count).Error

	if err != nil {
		return originalPayout, false, err
	}

	if count > 0 {
		if err := consumer(db, user, cards.DoubleDownCardID); err != nil {
			return originalPayout, false, err
		}
		return originalPayout * 2.0, true, nil
	}

	return originalPayout, false, nil
}

func ApplyEmotionalHedgeIfApplicable(db *gorm.DB, consumer CardConsumer, user models.User, bet models.Bet, userPick int, betAmount float64, scoreDiff int) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.EmotionalHedgeCardID).
		Count(&count).Error
	if err != nil {
		return 0, false, err
	}
	if count == 0 {
		return 0, false, nil
	}

	var guild models.Guild
	if err := db.Where("guild_id = ?", user.GuildID).First(&guild).Error; err != nil {
		return 0, false, err
	}
	if guild.SubscribedTeam == nil || *guild.SubscribedTeam == "" {
		return 0, false, nil
	}
	subscribedTeam := *guild.SubscribedTeam

	var userPickedTeamName string
	if userPick == 1 {
		userPickedTeamName = bet.Option1
	} else {
		userPickedTeamName = bet.Option2
	}

	userPickedTeamNameNormalized := common.GetSchoolName(userPickedTeamName)
	subscribedTeamNormalized := common.GetSchoolName(subscribedTeam)

	isBetOnSubscribedTeam := userPickedTeamNameNormalized == subscribedTeamNormalized

	if !isBetOnSubscribedTeam {
		isBetOnSubscribedTeam = (userPickedTeamName == subscribedTeam)
	}

	if !isBetOnSubscribedTeam {
		return 0, false, nil
	}

	if scoreDiff == 0 {
		return 0, true, nil
	}

	teamWonStraightUp := false
	if userPick == 1 {
		teamWonStraightUp = scoreDiff > 0
	} else {
		teamWonStraightUp = scoreDiff < 0
	}

	if !teamWonStraightUp {
		if err := consumer(db, user, cards.EmotionalHedgeCardID); err != nil {
			return 0, false, err
		}
		refund := betAmount * 0.5
		return refund, true, nil
	}

	return 0, true, nil
}

func ApplyBetInsuranceIfApplicable(db *gorm.DB, consumer CardConsumer, user models.User, betAmount float64, isWin bool) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.BetInsuranceCardID).
		Count(&count).Error

	if err != nil {
		return 0, false, err
	}

	if count > 0 {
		if err := consumer(db, user, cards.BetInsuranceCardID); err != nil {
			return 0, false, err
		}

		if !isWin {
			refund := betAmount * 0.25
			return refund, true, nil
		} else {
			return 0, true, nil
		}
	}

	return 0, false, nil
}

func ApplyGetOutOfJailIfApplicable(db *gorm.DB, consumer CardConsumer, user models.User, betAmount float64) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.GetOutOfJailCardID).
		Count(&count).Error

	if err != nil {
		return 0, false, err
	}

	if count > 0 {
		if err := consumer(db, user, cards.GetOutOfJailCardID); err != nil {
			return 0, false, err
		}

		return betAmount, true, nil
	}

	return 0, false, nil
}

func ApplyGamblerIfAvailable(db *gorm.DB, consumer CardConsumer, user models.User, originalPayout float64, isWin bool) (float64, bool, error) {
	var count int64
	err := db.Model(&models.UserInventory{}).
		Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, user.GuildID, cards.GamblerCardID).
		Count(&count).Error

	if err != nil {
		return originalPayout, false, err
	}

	if count > 0 {
		if err := consumer(db, user, cards.GamblerCardID); err != nil {
			return originalPayout, false, err
		}

		if rand.Intn(2) == 0 {
			return originalPayout * 2.0, true, nil
		}

		return originalPayout, true, nil
	}

	return originalPayout, false, nil
}

func processRoyaltyPayment(tx *gorm.DB, card *models.Card, royaltyGuildID string) error {
	if card.RoyaltyDiscordUserID == nil {
		return nil
	}

	var royaltyAmount float64
	if card.CardRarity.ID != 0 {
		royaltyAmount = card.CardRarity.Royalty
	} else {
		royaltyAmount = 0.5 // Default to Common royalty
	}

	var royaltyGuild models.Guild
	guildResult := tx.Where("guild_id = ?", royaltyGuildID).First(&royaltyGuild)
	if guildResult.Error != nil {
		return fmt.Errorf("error fetching royalty guild: %v", guildResult.Error)
	}

	var royaltyUser models.User
	result := tx.First(&royaltyUser, models.User{
		DiscordID: *card.RoyaltyDiscordUserID,
		GuildID:   royaltyGuildID,
	})
	if result.Error != nil {
		return fmt.Errorf("error fetching royalty user: %v", result.Error)
	}

	if err := tx.Model(&royaltyUser).UpdateColumn("points", gorm.Expr("points + ?", royaltyAmount)).Error; err != nil {
		return fmt.Errorf("error saving royalty user: %v", err)
	}

	return nil
}

func addCardToInventory(db *gorm.DB, userID uint, guildID string, cardID uint) error {
	inventory := models.UserInventory{
		UserID:  userID,
		GuildID: guildID,
		CardID:  cardID,
	}
	return db.Create(&inventory).Error
}

func processTagCards(tx *gorm.DB, guildID string) error {
	now := time.Now()
	expirationTime := now.Add(-12 * time.Hour)

	var tagCards []models.UserInventory
	if err := tx.Where("guild_id = ? AND card_id = ? AND deleted_at IS NULL", guildID, cards.TagCardID).
		Find(&tagCards).Error; err != nil {
		return err
	}

	if len(tagCards) == 0 {
		return nil
	}

	userPointsMap := make(map[uint]bool)
	var expiredCards []models.UserInventory
	var userIDsToUpdate []uint

	for _, tagCard := range tagCards {
		if tagCard.CreatedAt.Before(expirationTime) {
			expiredCards = append(expiredCards, tagCard)
		} else {
			if !userPointsMap[tagCard.UserID] {
				userIDsToUpdate = append(userIDsToUpdate, tagCard.UserID)
				userPointsMap[tagCard.UserID] = true
			}
		}
	}

	if len(userIDsToUpdate) > 0 {
		if err := tx.Model(&models.User{}).
			Where("id IN ? AND guild_id = ?", userIDsToUpdate, guildID).
			UpdateColumn("points", gorm.Expr("points + 1.0")).Error; err != nil {
			return err
		}
	}

	for _, expiredCard := range expiredCards {
		if err := tx.Delete(&expiredCard).Error; err != nil {
			return err
		}
	}

	return nil
}

func getUserInventory(db *gorm.DB, userID uint, guildID string) ([]models.UserInventory, error) {
	var inventory []models.UserInventory
	err := db.Where("user_id = ? AND guild_id = ?", userID, guildID).Find(&inventory).Error
	return inventory, err
}

func MyInventory(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	var user models.User
	result := db.Where(models.User{DiscordID: userID, GuildID: guildID}).Attrs(models.User{Points: guild.StartingPoints}).FirstOrCreate(&user)
	if result.Error != nil {
		common.SendError(s, i, fmt.Errorf("error fetching user: %v", result.Error), db)
		return
	}

	inventory, err := getUserInventory(db, user.ID, guildID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error fetching inventory: %v", err), db)
		return
	}

	cardCounts := make(map[uint]int)
	for _, item := range inventory {
		cardCounts[item.CardID]++
	}

	now := time.Now()
	expirationTime := now.Add(-12 * time.Hour)
	hasShoppingSpree := false
	for _, item := range inventory {
		if item.CardID == cards.ShoppingSpreeCardID {
			if item.CreatedAt.After(expirationTime) || item.CreatedAt.Equal(expirationTime) {
				hasShoppingSpree = true
				break
			}
		}
	}

	resetPeriod := time.Duration(guild.CardDrawCooldownMinutes) * time.Minute

	var countdownText string
	if user.FirstCardDrawCycle != nil {
		resetTime := user.FirstCardDrawCycle.Add(resetPeriod)
		if now.Before(resetTime) {
			timeRemaining := resetTime.Sub(now)
			hours := int(timeRemaining.Hours())
			minutes := int(timeRemaining.Minutes()) % 60
			seconds := int(timeRemaining.Seconds()) % 60
			if hours > 0 {
				countdownText = fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
			} else if minutes > 0 {
				countdownText = fmt.Sprintf("%dm %ds", minutes, seconds)
			} else {
				countdownText = fmt.Sprintf("%ds", seconds)
			}
		} else {
			countdownText = "Next Draw Resets"
		}
	} else {
		countdownText = "No draws yet"
	}

	var nextDrawCount int
	if user.FirstCardDrawCycle != nil {
		resetTime := user.FirstCardDrawCycle.Add(resetPeriod)
		if now.After(resetTime) || now.Equal(resetTime) {
			nextDrawCount = 0
		} else {
			nextDrawCount = user.CardDrawCount
		}
	} else {
		nextDrawCount = 0
	}

	var nextDrawCost float64
	switch nextDrawCount {
	case 0:
		nextDrawCost = guild.CardDrawCost
	case 1:
		nextDrawCost = guild.CardDrawCost * 10
	default:
		nextDrawCost = guild.CardDrawCost * 100
	}

	hasLuckyHorseshoe := cardCounts[cards.LuckyHorseshoeCardID] > 0
	if hasLuckyHorseshoe {
		nextDrawCost = nextDrawCost * 0.5
	}

	hasUnluckyCat := cardCounts[cards.UnluckyCatCardID] > 0
	if hasUnluckyCat {
		nextDrawCost = nextDrawCost * 2.0
	}

	hasCoupon := cardCounts[cards.CouponCardID] > 0
	if hasCoupon {
		nextDrawCost = nextDrawCost * 0.75
	}

	if hasShoppingSpree {
		nextDrawCost = nextDrawCost * 0.5
	}

	var lockoutText string
	if user.CardDrawTimeoutUntil != nil && now.Before(*user.CardDrawTimeoutUntil) {
		timeRemaining := user.CardDrawTimeoutUntil.Sub(now)
		hours := int(timeRemaining.Hours())
		minutes := int(timeRemaining.Minutes()) % 60
		seconds := int(timeRemaining.Seconds()) % 60
		if hours > 0 {
			lockoutText = fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
		} else if minutes > 0 {
			lockoutText = fmt.Sprintf("%dm %ds", minutes, seconds)
		} else {
			lockoutText = fmt.Sprintf("%ds", seconds)
		}
	}

	if len(cardCounts) == 0 {
		fields := []*discordgo.MessageEmbedField{
			{
				Name:   "⏱️ Timer Reset",
				Value:  countdownText,
				Inline: true,
			},
			{
				Name:   "💰 Next Draw Cost",
				Value:  fmt.Sprintf("%.0f points", nextDrawCost),
				Inline: true,
			},
		}

		if lockoutText != "" {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "🚫 Draw Lockout",
				Value:  lockoutText,
				Inline: true,
			})
		}

		embed := &discordgo.MessageEmbed{
			Title:       "🎴 Your Inventory",
			Description: "Your inventory is empty. Draw some cards to add them to your hand!",
			Color:       0x3498DB,
			Fields:      fields,
		}

		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

	rarityOrder := []string{"Mythic", "Epic", "Rare", "Uncommon", "Common"}
	cardsByRarity := make(map[string][]struct {
		Card  *models.Card
		Count int
	})

	for cardID, count := range cardCounts {
		card := GetCardByID(uint(cardID))
		if card == nil {
			continue
		}
		rarityName := "Common"
		if card.CardRarity.ID != 0 {
			rarityName = card.CardRarity.Name
		}
		if cardsByRarity[rarityName] == nil {
			cardsByRarity[rarityName] = []struct {
				Card  *models.Card
				Count int
			}{}
		}
		cardsByRarity[rarityName] = append(cardsByRarity[rarityName], struct {
			Card  *models.Card
			Count int
		}{Card: card, Count: count})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🎴 Your Inventory",
		Description: "Cards currently in your hand",
		Color:       0x3498DB,
		Fields:      []*discordgo.MessageEmbedField{},
	}

	for _, rarity := range rarityOrder {
		cardsHeld, exists := cardsByRarity[rarity]
		if !exists || len(cardsHeld) == 0 {
			continue
		}

		var fieldValue string
		for _, cardInfo := range cardsHeld {
			quantityText := ""
			if cardInfo.Count > 1 {
				quantityText = fmt.Sprintf(" (x%d)", cardInfo.Count)
			}
			fieldValue += fmt.Sprintf("**%s**%s\n%s\n\n", cardInfo.Card.Name, quantityText, cardInfo.Card.Description)
		}

		// Get emoji from first card in this rarity group
		var rarityEmoji string = "🤍" // Default to Common emoji
		if len(cardsHeld) > 0 && cardsHeld[0].Card.CardRarity.ID != 0 {
			rarityEmoji = cardsHeld[0].Card.CardRarity.Icon
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s", rarityEmoji, rarity),
			Value:  fieldValue,
			Inline: false,
		})
	}

	totalCards := 0
	for _, count := range cardCounts {
		totalCards += count
	}
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Total cards: %d", totalCards),
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "⏱️ Timer Reset",
		Value:  countdownText,
		Inline: true,
	})

	costText := fmt.Sprintf("%.0f points", nextDrawCost)
	if nextDrawCost == 0 {
		costText = "Free (Generous Donation)"
	} else if hasShoppingSpree {
		costText = fmt.Sprintf("%.0f points (Shopping Spree: -50%%)", nextDrawCost)
	}
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "💰 Next Draw Cost",
		Value:  costText,
		Inline: true,
	})

	if lockoutText != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🚫 Draw Lockout",
			Value:  lockoutText,
			Inline: true,
		})
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

func PlayCard(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	var user models.User
	result := db.Where(models.User{DiscordID: userID, GuildID: guildID}).Attrs(models.User{Points: guild.StartingPoints}).FirstOrCreate(&user)
	if result.Error != nil {
		common.SendError(s, i, fmt.Errorf("error fetching user: %v", result.Error), db)
		return
	}

	showPlayableCardSelectMenu(s, i, db, userID, guildID, user.ID)
}

func showPlayableCardSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, guildID string, userDBID uint) {
	inventory, err := getUserInventory(db, userDBID, guildID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error fetching inventory: %v", err), db)
		return
	}

	inventoryMap := make(map[uint]int)
	for _, item := range inventory {
		inventoryMap[item.CardID]++
	}

	var playableCards []struct {
		Card  *models.Card
		Count int
	}

	for cardID, count := range inventoryMap {
		card := GetCardByID(uint(cardID))
		if card != nil && card.UserPlayable {
			playableCards = append(playableCards, struct {
				Card  *models.Card
				Count int
			}{Card: card, Count: count})
		}
	}

	if len(playableCards) == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You don't have any playable cards in your inventory. Draw some cards to get started!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	maxOptions := 25
	if len(playableCards) > maxOptions {
		playableCards = playableCards[:maxOptions]
	}

	var selectOptions []discordgo.SelectMenuOption
	for _, pc := range playableCards {
		label := pc.Card.Name
		if pc.Count > 1 {
			label = fmt.Sprintf("%s (x%d)", pc.Card.Name, pc.Count)
		}

		if len(label) > 100 {
			label = label[:97] + "..."
		}

		description := pc.Card.Description
		if len(description) > 100 {
			description = description[:97] + "..."
		}

		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label:       label,
			Value:       fmt.Sprintf("%d", pc.Card.ID),
			Description: description,
			Emoji:       nil,
			Default:     false,
		})
	}

	minValues := 1
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Select a card to play from your inventory:",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("playcard_select_%s_%s", userID, guildID),
							Placeholder: "Choose a card to play...",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     selectOptions,
						},
					},
				},
			},
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
	}
}
