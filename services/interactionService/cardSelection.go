package interactionService

import (
	"fmt"
	"math/rand"
	"perfectOddsBot/models"
	cardService "perfectOddsBot/services/cardService"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/guildService"
	cardSelection "perfectOddsBot/services/interactionService/cardSelection"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func HandleCardUserSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	customID := i.MessageComponentData().CustomID
	var err error

	if !cardService.TryMarkSelectorUsed(customID) {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ This card has already been played. You can only use each card once.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	parts := strings.Split(customID, "_")
	if len(parts) != 6 {
		return fmt.Errorf("invalid card selection custom ID format")
	}

	cardID, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}

	userID := parts[3]
	guildID := parts[4]

	if i.Member.User.ID != userID {
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only select a target for your own card draw.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no user selected")
	}
	targetUserID := i.MessageComponentData().Values[0]

	if targetUserID == userID {
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You cannot target yourself!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			return err
		}
		return nil
	}
	switch cardID {
	case cards.PettyTheftCardID:
		err = cardSelection.HandlePettyTheftSelection(s, i, db, userID, targetUserID, guildID)
	case cards.JesterCardID:
		err = cardSelection.HandleJesterSelection(s, i, db, userID, targetUserID, guildID)
	case cards.BetFreezeCardID:
		err = cardSelection.HandleBetFreezeSelection(s, i, db, userID, targetUserID, guildID)
	case cards.GrandLarcenyCardID:
		err = cardSelection.HandleGrandLarcenySelection(s, i, db, userID, targetUserID, guildID)
	case cards.AntiAntiBetCardID:
		err = cardSelection.HandleAntiAntiBetSelection(s, i, db, userID, targetUserID, guildID)
	case cards.HostileTakeoverCardID:
		err = cardSelection.HandleHostileTakeoverSelection(s, i, db, userID, targetUserID, guildID)
	case cards.JusticeCardID:
		err = cardSelection.HandleJusticeSelection(s, i, db, userID, targetUserID, guildID)
	case cards.TheGossipCardID:
		err = cardSelection.HandleTheGossipSelection(s, i, db, userID, targetUserID, guildID)
	case cards.DuelCardID:
		err = cardSelection.HandleDuelSelection(s, i, db, userID, targetUserID, guildID)
	case cards.TagCardID:
		err = cardSelection.HandleTagSelection(s, i, db, userID, targetUserID, guildID)
	case cards.BountyHunterCardID:
		err = cardSelection.HandleBountyHunterSelection(s, i, db, userID, targetUserID, guildID)
	case cards.SocialDistancingCardID:
		err = cardSelection.HandleSocialDistancingSelection(s, i, db, userID, targetUserID, guildID)
	case cards.TheLoversCardID:
		err = cardSelection.HandleTheLoversSelection(s, i, db, userID, targetUserID, guildID)
	case cards.TheHighPriestessCardID:
		err = cardSelection.HandleTheHighPriestessSelection(s, i, db, userID, targetUserID, guildID)
	case cards.TheMagicianCardID:
		err = cardSelection.HandleTheMagicianSelection(s, i, db, userID, targetUserID, guildID)
	default:
		cardService.UnmarkSelectorUsed(customID)
		return fmt.Errorf("card %d does not support user selection", cardID)
	}

	if err != nil {
		cardService.UnmarkSelectorUsed(customID)
	}

	return err
}



func HandleCardBetSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	customID := i.MessageComponentData().CustomID

	if !cardService.TryMarkSelectorUsed(customID) {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ This card has already been played. You can only use each card once.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	parts := strings.Split(customID, "_")
	if len(parts) != 6 {
		return fmt.Errorf("invalid card bet selection custom ID format")
	}

	cardIDInt, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}
	cardID := uint(cardIDInt)

	userID := parts[3]
	guildID := parts[4]

	if i.Member.User.ID != userID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only make selections for your own card draw.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no bet selected")
	}
	targetBetIDStr := i.MessageComponentData().Values[0]
	targetBetIDVal, err := strconv.ParseUint(targetBetIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid bet ID: %v", err)
	}
	targetBetID := uint(targetBetIDVal)

	err = db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		inventory := models.UserInventory{
			UserID:      user.ID,
			GuildID:     guildID,
			CardID:      cardID,
			TargetBetID: &targetBetID,
		}

		if err := tx.Create(&inventory).Error; err != nil {
			return err
		}

		card := cardService.GetCardByID(cardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		var bet models.Bet
		if err := tx.First(&bet, targetBetID).Error; err != nil {
			return fmt.Errorf("bet not found")
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		embed := cardSelection.BuildCardResultEmbed(card, &models.CardResult{
			Message:     fmt.Sprintf("Uno Reverse card active! If you lose on '%s', you win (and vice versa)!", bet.Description),
			PointsDelta: 0,
			PoolDelta:   0,
		}, user, "", guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})

	if err != nil {
		cardService.UnmarkSelectorUsed(customID)
	}

	return err
}

func HandleCardOptionSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	customID := i.MessageComponentData().CustomID

	if !cardService.TryMarkSelectorUsed(customID) {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ This card has already been played. You can only use each card once.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	parts := strings.Split(customID, "_")
	if len(parts) != 6 {
		return fmt.Errorf("invalid card option selection custom ID format")
	}

	cardIDInt, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}
	cardID := uint(cardIDInt)

	userID := parts[3]
	guildID := parts[4]

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no option selected")
	}
	selectedOptionIDStr := i.MessageComponentData().Values[0]
	selectedOptionID, err := strconv.Atoi(selectedOptionIDStr)
	if err != nil {
		return fmt.Errorf("invalid option ID: %v", err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		card := cardService.GetCardByID(cardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		if cardID == cards.TheWheelOfFortuneCardID {
			var result *models.CardResult

			if selectedOptionID == cards.WheelOptionDeflation {
				poolLoss := guild.Pool * 0.5
				guild.Pool -= poolLoss
				if guild.Pool < 0 {
					guild.Pool = 0
				}
				if err := tx.Save(&guild).Error; err != nil {
					return err
				}

				result = &models.CardResult{
					Message:     fmt.Sprintf("<@%s> chose **Deflation**! The pool lost 50%% of its total points (%.0f points).", user.DiscordID, poolLoss),
					PointsDelta: 0,
					PoolDelta:   -poolLoss,
				}
			} else if selectedOptionID == cards.WheelOptionChaos {
				var allUsers []models.User
				if err := tx.Where("guild_id = ? AND deleted_at IS NULL", guildID).Find(&allUsers).Error; err != nil {
					return err
				}

				messageDetails := ""
				shown := 0
				const maxLines = 25

				for i := range allUsers {
					pointsChange := float64(rand.Intn(500) + 1)
					if rand.Intn(2) == 0 {
						pointsChange = -pointsChange
					}

					allUsers[i].Points += pointsChange
					if allUsers[i].Points < 0 {
						allUsers[i].Points = 0
					}

					if err := tx.Save(&allUsers[i]).Error; err != nil {
						return err
					}

					if shown < maxLines {
						username := allUsers[i].Username
						displayName := ""
						if username == nil || *username == "" {
							displayName = fmt.Sprintf("<@%s>", allUsers[i].DiscordID)
						} else {
							displayName = *username
						}

						sign := "+"
						if pointsChange < 0 {
							sign = ""
						}
						messageDetails += fmt.Sprintf("%s: %s%.0f points\n", displayName, sign, pointsChange)
						shown++
					}
				}

				if len(allUsers) > maxLines {
					messageDetails += fmt.Sprintf("\n...and %d more.", len(allUsers)-maxLines)
				}

				result = &models.CardResult{
					Message:     fmt.Sprintf("<@%s> chose **Chaos**! Every player randomly gained or lost 1-500 points:\n\n%s", user.DiscordID, messageDetails),
					PointsDelta: 0,
					PoolDelta:   0,
				}
			} else {
				return fmt.Errorf("invalid option ID for The Wheel of Fortune")
			}

			embed := cardSelection.BuildCardResultEmbed(card, result, user, "", guild.Pool)
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{embed},
				},
			})
		}

		if selectedOptionID == cards.GamblerOptionNo {
			var inventory models.UserInventory
			result := tx.Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, guildID, cardID).First(&inventory)
			if result.Error == nil {
				if err := tx.Delete(&inventory).Error; err != nil {
					return fmt.Errorf("error removing card from inventory: %v", err)
				}
			}

			embed := cardSelection.BuildCardResultEmbed(card, &models.CardResult{
				Message:     fmt.Sprintf("<@%s> chose 'No'. The card fizzles out and is not added to your inventory.", user.DiscordID),
				PointsDelta: 0,
				PoolDelta:   0,
			}, user, "", guild.Pool)

			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{embed},
				},
			})
		}

		embed := cardSelection.BuildCardResultEmbed(card, &models.CardResult{
			Message:     fmt.Sprintf("<@%s> chose 'Yes'! The Gambler has been added to your inventory. Your next bet resolution has a 50/50 chance to double your win or loss.", user.DiscordID),
			PointsDelta: 0,
			PoolDelta:   0,
		}, user, "", guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})

	if err != nil {
		cardService.UnmarkSelectorUsed(customID)
	}

	return err
}

func HandlePlayCardSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, "_")
	if len(parts) != 4 {
		return fmt.Errorf("invalid playcard selection custom ID format")
	}

	userID := parts[2]
	guildID := parts[3]

	if i.Member.User.ID != userID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only play your own cards.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no card selected")
	}

	selectedCardIDStr := i.MessageComponentData().Values[0]
	selectedCardIDInt, err := strconv.Atoi(selectedCardIDStr)
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}
	selectedCardID := uint(selectedCardIDInt)

	if selectedCardID == cards.StopTheStealCardID {
		cardSelection.ShowBetSelectMenuForPlayCard(s, i, db, selectedCardID, userID, guildID)
		return nil
	}

	if selectedCardID == cards.PoolBoyCardID {
		return cardSelection.HandlePoolBoyPlay(s, i, db, selectedCardID, userID, guildID)
	}

	if selectedCardID == cards.TheEmperorCardID {
		return cardSelection.HandleEmperorPlay(s, i, db, selectedCardID, userID, guildID)
	}

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "This card type is not yet supported for manual play.",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}


func HandlePlayCardBetSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	customID := i.MessageComponentData().CustomID
	parts := strings.Split(customID, "_")
	if len(parts) != 5 {
		return fmt.Errorf("invalid playcard bet selection custom ID format")
	}

	cardIDInt, err := strconv.Atoi(parts[2])
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}
	cardID := uint(cardIDInt)

	userID := parts[3]
	guildID := parts[4]

	if i.Member.User.ID != userID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only play your own cards.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no bet selected")
	}

	selectedBetIDStr := i.MessageComponentData().Values[0]
	selectedBetID, err := strconv.ParseUint(selectedBetIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid bet ID: %v", err)
	}
	betID := uint(selectedBetID)

	return db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return fmt.Errorf("user not found: %v", err)
		}

		var bet models.Bet
		if err := tx.First(&bet, "id = ? AND guild_id = ? AND paid = 0 AND deleted_at IS NULL", betID, guildID).Error; err != nil {
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "This bet is no longer available for cancellation.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}

		var entries []models.BetEntry
		if err := tx.Where("bet_id = ? AND user_id = ? AND deleted_at IS NULL", betID, user.ID).Find(&entries).Error; err != nil {
			return fmt.Errorf("error querying bet entries: %v", err)
		}

		if len(entries) == 0 {
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "No bet entries found to cancel.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}

		refundAmount := 0
		for _, entry := range entries {
			refundAmount += entry.Amount
		}

		user.Points += float64(refundAmount)
		if err := tx.Save(&user).Error; err != nil {
			return fmt.Errorf("error refunding points: %v", err)
		}

		result := tx.Where("bet_id = ? AND user_id = ? AND deleted_at IS NULL", betID, user.ID).Delete(&models.BetEntry{})
		if result.Error != nil {
			return fmt.Errorf("error soft deleting bet entries: %v", result.Error)
		}

		card := cardService.GetCardByID(cardID)
		if card == nil {
			return fmt.Errorf("card not found: %d", cardID)
		}

		if err := cardService.PlayCardFromInventoryWithMessage(s, tx, user, cardID, fmt.Sprintf("<@%s> played **%s** and cancelled bet: **%s**", userID, card.Name, bet.Description)); err != nil {
			return fmt.Errorf("error consuming card: %v", err)
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return fmt.Errorf("error getting guild info: %v", err)
		}

		embed := cardSelection.BuildCardResultEmbed(card, &models.CardResult{
			Message:     fmt.Sprintf("You cancelled your bet: **%s** and received a refund of **%d** points.", bet.Description, refundAmount),
			PointsDelta: float64(refundAmount),
			PoolDelta:   0,
		}, user, "", guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
	})
}

