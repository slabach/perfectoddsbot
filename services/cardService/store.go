package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)
func ShowStore(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
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

	var purchasableCards []models.Card
	err = db.Where("store_cost IS NOT NULL AND active = ?", true).
		Preload("CardRarity").
		Order("rarity_id, name").
		Find(&purchasableCards).Error
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error fetching purchasable cards: %v", err), db)
		return
	}

	if len(purchasableCards) == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "There are no cards available for purchase in the store right now.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	maxOptions := 25
	if len(purchasableCards) > maxOptions {
		purchasableCards = purchasableCards[:maxOptions]
	}

	rarityOrder := []string{"Mythic", "Epic", "Rare", "Uncommon", "Common"}
	cardsByRarity := make(map[string][]models.Card)
	for _, card := range purchasableCards {
		rarityName := "Common"
		if card.CardRarity.ID != 0 {
			rarityName = card.CardRarity.Name
		}
		cardsByRarity[rarityName] = append(cardsByRarity[rarityName], card)
	}

	const discordFieldValueLimit = 1024
	var fields []*discordgo.MessageEmbedField
	for _, rarity := range rarityOrder {
		cardsInRarity, exists := cardsByRarity[rarity]
		if !exists || len(cardsInRarity) == 0 {
			continue
		}

		var fieldValue string
		for _, card := range cardsInRarity {
			cost := 0.0
			if card.StoreCost != nil {
				cost = *card.StoreCost
			}
			cardEntry := fmt.Sprintf("**%s** - %.0f points\n%s\n\n", card.Name, cost, card.Description)
			// Truncate fieldValue to Discord's 1024 character limit per field
			if len(fieldValue)+len(cardEntry) > discordFieldValueLimit {
				// If adding this card would exceed the limit, truncate and break
				remaining := discordFieldValueLimit - len(fieldValue)
				if remaining > 0 {
					fieldValue += cardEntry[:remaining]
				}
				break
			}
			fieldValue += cardEntry
		}

		// Ensure fieldValue doesn't exceed limit (safety check)
		if len(fieldValue) > discordFieldValueLimit {
			fieldValue = fieldValue[:discordFieldValueLimit]
		}

		var rarityEmoji string = "🤍"
		if len(cardsInRarity) > 0 && cardsInRarity[0].CardRarity.ID != 0 {
			rarityEmoji = cardsInRarity[0].CardRarity.Icon
		}

		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s", rarityEmoji, rarity),
			Value:  fieldValue,
			Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🛒 Card Store",
		Description: "Purchase specific cards directly from the store. Points spent go into the pool.",
		Fields:      fields,
		Color:       0x3498DB,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Your points: %.1f", user.Points),
		},
	}

	var selectOptions []discordgo.SelectMenuOption
	for _, card := range purchasableCards {
		cost := 0.0
		if card.StoreCost != nil {
			cost = *card.StoreCost
		}

		label := card.Name
		if len(label) > 100 {
			label = label[:97] + "..."
		}

		description := fmt.Sprintf("%.0f points", cost)
		if len(card.Description) > 80 {
			description = fmt.Sprintf("%.0f points - %s", cost, card.Description[:77]+"...")
		} else {
			description = fmt.Sprintf("%.0f points - %s", cost, card.Description)
		}
		if len(description) > 100 {
			description = description[:97] + "..."
		}

		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label:       label,
			Value:       fmt.Sprintf("%d", card.ID),
			Description: description,
		})
	}

	minValues := 1
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("store_select_%s_%s", userID, guildID),
							Placeholder: "Choose a card to purchase...",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     selectOptions,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Purchase Selected Card",
							Style:    discordgo.PrimaryButton,
							CustomID: fmt.Sprintf("store_purchase_%s_%s", userID, guildID),
							Emoji:    &discordgo.ComponentEmoji{Name: "🛒"},
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

func ProcessStorePurchase(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, cardID uint, userID string, guildID string) error {
	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return err
	}

	if !guild.CardDrawingEnabled {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Card drawing is currently disabled for this server.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	var user models.User
	result := db.FirstOrCreate(&user, models.User{DiscordID: userID, GuildID: guildID})
	if result.Error != nil {
		common.SendError(s, i, fmt.Errorf("error fetching user: %v", result.Error), db)
		return result.Error
	}
	if result.RowsAffected == 1 {
		user.Points = guild.StartingPoints
		db.Save(&user)
	}

	username := common.GetUsernameFromUser(i.Member.User)
	common.UpdateUserUsername(db, &user, username)

	now := time.Now()

	resetPeriod := time.Duration(guild.CardDrawCooldownMinutes) * time.Minute
	if user.FirstCardDrawCycle != nil {
		timeSinceFirstDraw := now.Sub(*user.FirstCardDrawCycle)
		if timeSinceFirstDraw >= resetPeriod {
			user.FirstCardDrawCycle = &now
			user.CardDrawCount = 0
		}
	} else {
		user.FirstCardDrawCycle = &now
		user.CardDrawCount = 0
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var dbCard models.Card
	err = tx.Where("id = ? AND store_cost IS NOT NULL AND active = ?", cardID, true).
		Preload("CardRarity").
		Preload("Options").
		First(&dbCard).Error
	if err != nil {
		tx.Rollback()
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This card is not available for purchase or no longer exists.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	card := GetCardByID(cardID)
	if card == nil {
		tx.Rollback()
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Card not found in registry.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	storeCost := 0.0
	if dbCard.StoreCost != nil {
		storeCost = *dbCard.StoreCost
	}

	if storeCost <= 0 {
		tx.Rollback()
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This card has no store cost set.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	var lockedUser models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedUser, user.ID).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error locking user: %v", err), db)
		return err
	}

	if lockedUser.CardDrawTimeoutUntil != nil && now.Before(*lockedUser.CardDrawTimeoutUntil) {
		tx.Rollback()
		timeRemaining := lockedUser.CardDrawTimeoutUntil.Sub(now)
		minutesRemaining := int(timeRemaining.Minutes())
		secondsRemaining := int(timeRemaining.Seconds()) % 60
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("You are timed out from drawing cards. Time remaining: %d minutes and %d seconds.", minutesRemaining, secondsRemaining),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	if lockedUser.CardDrawTimeoutUntil != nil && now.After(*lockedUser.CardDrawTimeoutUntil) {
		lockedUser.CardDrawTimeoutUntil = nil
	}

	if lockedUser.Points < storeCost {
		tx.Rollback()
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("You need at least %.0f points to purchase this card. You have %.1f points.", storeCost, lockedUser.Points),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	user = lockedUser

	var lockedGuild models.Guild
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedGuild, guild.ID).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error locking guild: %v", err), db)
		return err
	}

	*guild = lockedGuild

	user.Points -= storeCost
	guild.Pool += storeCost

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

	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return err
	}

	if err := tx.Save(&guild).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return err
	}

	if err := processRoyaltyPayment(tx, card, cards.RoyaltyGuildID); err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error processing royalty payment: %v", err), db)
		return err
	}

	if card.AddToInventory || card.UserPlayable {
		if err := addCardToInventory(tx, user.ID, guildID, card.ID); err != nil {
			tx.Rollback()
			common.SendError(s, i, fmt.Errorf("error adding card to inventory: %v", err), db)
			return err
		}
	}

	cardResult, err := card.Handler(s, tx, userID, guildID)
	if err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error executing card effect: %v", err), db)
		return err
	}

	// Reload user to capture any changes made by the card handler
	if err := tx.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error reloading user after card effect: %v", err), db)
		return err
	}

	if err := processTagCards(tx, guildID); err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error processing tag cards: %v", err), db)
		return err
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
				return err
			}

			user.Points += cardResult.PointsDelta
			if user.Points < 0 {
				user.Points = 0
			}
			guild.Pool += cardResult.PoolDelta

			if err := tx.Save(&user).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return err
			}
			if err := tx.Save(&guild).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return err
			}

			tx.Commit()
			return nil
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
				return err
			}
			if err := tx.Save(&guild).Error; err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return err
			}

			tx.Commit()
			return nil
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
			return err
		}
		if err := tx.Save(&guild).Error; err != nil {
			tx.Rollback()
			common.SendError(s, i, err, db)
			return err
		}

		tx.Commit()
		return nil
	}

	if cardResult.PointsDelta < 0 {
		hasMoon, err := hasMoonInInventory(tx, user.ID, guildID)
		if err != nil {
			tx.Rollback()
			common.SendError(s, i, err, db)
			return err
		}
		if hasMoon {
			if err := PlayCardFromInventory(s, tx, user, cards.TheMoonCardID); err != nil {
				tx.Rollback()
				common.SendError(s, i, err, db)
				return err
			}
			randomUserID, err := getRandomUserForMoonFromCards(tx, guildID, []uint{user.ID})
			if err != nil {
				hasShield, err := hasShieldInInventory(tx, user.ID, guildID)
				if err != nil {
					tx.Rollback()
					common.SendError(s, i, err, db)
					return err
				}
				if hasShield {
					if err := PlayCardFromInventory(s, tx, user, cards.ShieldCardID); err != nil {
						tx.Rollback()
						common.SendError(s, i, err, db)
						return err
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
					return err
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
				return err
			}
			if hasShield {
				if err := PlayCardFromInventory(s, tx, user, cards.ShieldCardID); err != nil {
					tx.Rollback()
					common.SendError(s, i, err, db)
					return err
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
					return err
				}
				if hasMoon {
					if err := PlayCardFromInventory(s, tx, targetUser, cards.TheMoonCardID); err != nil {
						tx.Rollback()
						common.SendError(s, i, err, db)
						return err
					}
					randomUserID, err := getRandomUserForMoonFromCards(tx, guildID, []uint{targetUser.ID, user.ID})
					if err != nil {
						hasShield, err := hasShieldInInventory(tx, targetUser.ID, guildID)
						if err != nil {
							tx.Rollback()
							common.SendError(s, i, err, db)
							return err
						}
						if hasShield {
							if err := PlayCardFromInventory(s, tx, targetUser, cards.ShieldCardID); err != nil {
								tx.Rollback()
								common.SendError(s, i, err, db)
								return err
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
							return err
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
						return err
					}
					if hasShield {
						if err := PlayCardFromInventory(s, tx, targetUser, cards.ShieldCardID); err != nil {
							tx.Rollback()
							common.SendError(s, i, err, db)
							return err
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
					return err
				}
			}
			targetUsername = common.GetUsernameWithDB(db, s, guildID, *cardResult.TargetUserID)
		}
	}

	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return err
	}
	if err := tx.Save(&guild).Error; err != nil {
		tx.Rollback()
		common.SendError(s, i, err, db)
		return err
	}

	tx.Commit()

	username = common.GetUsernameWithDB(db, s, guildID, user.DiscordID)
	embed := buildCardEmbed(card, cardResult, user, username, targetUsername, guild.Pool, storeCost)

	embed.Title = strings.Replace(embed.Title, " Drew: ", " Purchased: ", 1)
	if embed.Footer != nil {
		embed.Footer.Text = fmt.Sprintf("Store Purchase: -%.0f points | Added %.0f to pool", storeCost, storeCost)
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return err
	}

	return nil
}
