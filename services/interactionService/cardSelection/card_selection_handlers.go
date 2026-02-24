package cardSelection

import (
	"fmt"
	"math"
	"perfectOddsBot/models"
	cardService "perfectOddsBot/services/cardService"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func HandlePettyTheftSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecutePickpocketSteal(tx, userID, targetUserID, guildID, 50.0)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.PettyTheftCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleJesterSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecuteJesterMute(s, tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.JesterCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleBetFreezeSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecuteBetFreeze(s, tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.BetFreezeCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleGrandLarcenySelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecutePickpocketSteal(tx, userID, targetUserID, guildID, 150.0)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.GrandLarcenyCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		if result.PointsDelta > 0 {
			targetName := targetUsername
			if result.TargetUserID != nil {
				targetName = fmt.Sprintf("<@%s>", *result.TargetUserID)
			}
			result.Message = fmt.Sprintf("Grand Larceny successful! <@%s> stole %.0f points from %s!", userID, result.PointsDelta, targetName)
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleTheGossipSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecuteTheGossip(s, tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.TheGossipCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleDuelSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecuteDuel(tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.DuelCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleTagSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecuteTag(tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.TagCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleAlleyOopSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecuteAlleyOop(tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.AlleyOopCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleTransferPortalSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var drawer models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&drawer).Error; err != nil {
			return err
		}
		var targetUser models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
			First(&targetUser).Error; err != nil {
			return err
		}

		drawerItems, err := cardService.GetTradeableInventoryForUser(tx, drawer.ID, guildID)
		if err != nil {
			return err
		}
		targetItems, err := cardService.GetTradeableInventoryForUser(tx, targetUser.ID, guildID)
		if err != nil {
			return err
		}
		if len(drawerItems) == 0 || len(targetItems) == 0 {
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "You or the target no longer have tradeable cards. This card fizzles out.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}

		fromDrawer := cardService.PickWeightedTradeable(drawerItems)
		fromTarget := cardService.PickWeightedTradeable(targetItems)
		if fromDrawer == nil || fromTarget == nil {
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Could not pick cards to swap. This card fizzles out.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}

		fromDrawer.Inventory.UserID = targetUser.ID
		fromTarget.Inventory.UserID = drawer.ID
		if err := tx.Save(&fromDrawer.Inventory).Error; err != nil {
			return err
		}
		if err := tx.Save(&fromTarget.Inventory).Error; err != nil {
			return err
		}

		card := cardService.GetCardByID(cards.TransferPortalCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}
		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)
		drawerCardName := "Unknown"
		targetCardName := "Unknown"
		if fromDrawer.Card != nil {
			drawerCardName = fromDrawer.Card.Name
		}
		if fromTarget.Card != nil {
			targetCardName = fromTarget.Card.Name
		}
		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}
		result := &models.CardResult{
			Message:     fmt.Sprintf("<@%s> swapped **%s** for **%s** with %s!", drawer.DiscordID, drawerCardName, targetCardName, targetUsername),
			PointsDelta: 0,
			PoolDelta:   0,
		}
		embed := BuildCardResultEmbed(card, result, drawer, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleBlindsideBlockSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecuteBlindsideBlock(tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.BlindsideBlockCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleBracketBusterSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var targetUser models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
			First(&targetUser).Error; err != nil {
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Target user not found in this server.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}

		var activeBetIDs []uint
		if err := tx.Model(&models.Bet{}).Where("guild_id = ? AND paid = ? AND deleted_at IS NULL", guildID, false).Pluck("id", &activeBetIDs).Error; err != nil {
			return err
		}
		if len(activeBetIDs) == 0 {
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "There are no active (unpaid) bets in this server. Bracket Buster fizzles out.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}

		var entry models.BetEntry
		if err := tx.Where("user_id = ? AND bet_id IN ? AND deleted_at IS NULL", targetUser.ID, activeBetIDs).
			Order("amount ASC").
			First(&entry).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: fmt.Sprintf("<@%s> has no active bet entries. Bracket Buster fizzles out.", targetUserID),
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			}
			return err
		}

		wagerAmount := float64(entry.Amount)
		var bet models.Bet
		if err := tx.First(&bet, entry.BetID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&entry).Error; err != nil {
			return err
		}

		var guild models.Guild
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
			return err
		}
		guild.Pool += wagerAmount
		if err := tx.Save(&guild).Error; err != nil {
			return err
		}

		var drawer models.User
		if err := tx.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&drawer).Error; err != nil {
			return err
		}
		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)
		card := cardService.GetCardByID(cards.BracketBusterCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}
		result := &models.CardResult{
			Message:     fmt.Sprintf("Bracket Buster! <%s> cancelled <@%s>'s smallest active bet on \"%s\" (%.0f points). %.0f points have been added to the pool.", userID, targetUserID, bet.Description, wagerAmount, wagerAmount),
			PointsDelta: 0,
			PoolDelta:   wagerAmount,
		}
		embed := BuildCardResultEmbed(card, result, drawer, targetUsername, guild.Pool)
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleBountyHunterSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecuteBountyHunter(tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.BountyHunterCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleSocialDistancingSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecuteSocialDistancing(tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.SocialDistancingCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleAntiAntiBetSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		var betAmount float64
		if user.Points >= 100.0 {
			betAmount = 100.0
		} else {
			betAmount = math.Round(user.Points / 2.0)
		}

		user.Points -= betAmount
		if user.Points < 0 {
			user.Points = 0
		}

		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		inventory := models.UserInventory{
			UserID:       user.ID,
			GuildID:      guildID,
			CardID:       cards.AntiAntiBetCardID,
			TargetUserID: &targetUserID,
			BetAmount:    betAmount,
		}

		if err := tx.Create(&inventory).Error; err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.AntiAntiBetCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, &models.CardResult{
			Message:     fmt.Sprintf("Anti-Anti-Bet active! <@%s> bet %.0f points that <@%s> will lose their next bet. If they lose, they'll get %.0f points at even odds (+100).", user.DiscordID, betAmount, targetUserID, betAmount*2),
			PointsDelta: -betAmount,
			PoolDelta:   0,
		}, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleHostileTakeoverSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var drawer models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&drawer).Error; err != nil {
			return err
		}

		var target models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
			First(&target).Error; err != nil {
			return err
		}

		pointDiff := math.Abs(drawer.Points - target.Points)
		if pointDiff > 500.0 {
			if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error: Selected user is not within 500 points of you. The takeover cannot be completed.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}); err != nil {
				return err
			}

			return fmt.Errorf("selected user is not within 500 points")
		}

		result, err := cards.ExecuteHostileTakeover(tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var drawerAfter, targetAfter models.User
		tx.First(&drawerAfter, drawer.ID)
		tx.First(&targetAfter, target.ID)
		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)
		if result.TargetUserID != nil {
			targetUsername = common.GetUsernameWithDB(tx, s, guildID, *result.TargetUserID)
		}

		card := cardService.GetCardByID(cards.HostileTakeoverCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, drawerAfter, targetUsername, guild.Pool)

		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "You",
				Value:  fmt.Sprintf("<@%s>: %.1f → %.1f points", drawerAfter.DiscordID, drawer.Points, drawerAfter.Points),
				Inline: true,
			},
			{
				Name:   "Target",
				Value:  fmt.Sprintf("<@%s>: %.1f → %.1f points", targetAfter.DiscordID, target.Points, targetAfter.Points),
				Inline: true,
			},
		}

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleJusticeSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var drawer models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&drawer).Error; err != nil {
			return err
		}

		var target models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).
			First(&target).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Target user not found in this server.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			}
			return err
		}

		pointDiff := math.Abs(drawer.Points - target.Points)
		if pointDiff > 500.0 {
			if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error: Selected user is not within 500 points of you. Justice cannot be served.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			}); err != nil {
				return err
			}
			return fmt.Errorf("selected user is not within 500 points")
		}

		result, err := cards.ExecuteJustice(s, tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.JusticeCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleTheLoversSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		result, err := cards.ExecuteTheLovers(tx, userID, targetUserID, guildID)
		if err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.TheLoversCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := BuildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func HandleTheHighPriestessSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var targetUser models.User
		if err := tx.Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).First(&targetUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Target user not found in this server.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			}
			return err
		}

		var inventory []models.UserInventory
		if err := tx.Where("user_id = ? AND guild_id = ? AND deleted_at IS NULL", targetUser.ID, guildID).Find(&inventory).Error; err != nil {
			return err
		}

		cardCounts := make(map[uint]int)
		for _, item := range inventory {
			cardCounts[item.CardID]++
		}

		targetMention := "<@" + targetUserID + ">"

		if len(cardCounts) == 0 {
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:       "🔮 The High Priestess - Hidden Knowledge",
							Description: fmt.Sprintf("The High Priestess reveals that %s's inventory is empty.", targetMention),
							Color:       0x9B59B6,
						},
					},
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})
		}

		rarityOrder := []string{"Mythic", "Epic", "Rare", "Uncommon", "Common"}
		cardsByRarity := make(map[string][]struct {
			Card  *models.Card
			Count int
		})

		for cardID, count := range cardCounts {
			card := cardService.GetCardByID(cardID)
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

		var fields []*discordgo.MessageEmbedField
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
				fieldValue += fmt.Sprintf("• %s%s\n", cardInfo.Card.Name, quantityText)
			}

			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   rarity,
				Value:  fieldValue,
				Inline: false,
			})
		}

		embed := &discordgo.MessageEmbed{
			Title:       "🔮 The High Priestess - Hidden Knowledge",
			Description: fmt.Sprintf("The High Priestess reveals %s's inventory:", targetMention),
			Color:       0x9B59B6,
			Fields:      fields,
		}

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
	})
}

func HandleTheMagicianSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var targetUser models.User
		if err := tx.Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).First(&targetUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Target user not found in this server.",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
			}
			return err
		}

		var inventory []models.UserInventory
		if err := tx.Where("user_id = ? AND guild_id = ? AND deleted_at IS NULL", targetUser.ID, guildID).Find(&inventory).Error; err != nil {
			return err
		}

		targetMention := "<@" + targetUserID + ">"

		var eligibleItems []models.UserInventory
		for _, item := range inventory {
			card := cardService.GetCardByID(item.CardID)
			if card == nil {
				continue
			}

			if card.CardRarity.ID != 0 && card.CardRarity.Name == "Mythic" {
				continue
			}
			eligibleItems = append(eligibleItems, item)
		}

		if len(eligibleItems) == 0 {
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("The Magician (<@%s>) tried to borrow from %s, but they have no eligible cards (Mythic cards are excluded). The card fizzles out.", userID, targetMention),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}

		var selectOptions []discordgo.SelectMenuOption
		for _, item := range eligibleItems {
			card := cardService.GetCardByID(item.CardID)
			if card == nil {
				continue
			}

			label := card.Name
			if len(label) > 100 {
				label = label[:97] + "..."
			}

			description := card.Description
			if len(description) > 100 {
				description = description[:97] + "..."
			}

			value := fmt.Sprintf("%d_%d_%s_%s_%s", item.ID, item.CardID, userID, targetUserID, guildID)
			if len(value) > 100 {
				value = value[:100]
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
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("The Magician (<@%s>) tried to borrow from %s, but they have no eligible cards. The card fizzles out.", userID, targetMention),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
		}

		var paginatedOptions [][]discordgo.SelectMenuOption
		minValues := 1
		for i := 0; i < len(selectOptions); i += 25 {
			end := i + 25
			if end > len(selectOptions) {
				end = len(selectOptions)
			}
			paginatedOptions = append(paginatedOptions, selectOptions[i:end])
		}

		selectorID := MagicianSelectorID()
		currentPage := 0
		components := []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						MenuType:    discordgo.StringSelectMenu,
						CustomID:    fmt.Sprintf("magician_card_select_%s_%s_%s_%s", selectorID, userID, targetUserID, guildID),
						Placeholder: fmt.Sprintf("Select a card to borrow from %s", targetMention),
						MinValues:   &minValues,
						MaxValues:   1,
						Options:     paginatedOptions[currentPage],
					},
				},
			},
		}

		if len(paginatedOptions) > 1 {
			components = append(components, discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Previous",
						CustomID: fmt.Sprintf("magician_card_prev_%d_%s_%s_%s_%s", currentPage, selectorID, userID, targetUserID, guildID),
						Style:    discordgo.PrimaryButton,
						Disabled: currentPage == 0,
					},
					discordgo.Button{
						Label:    "Next",
						CustomID: fmt.Sprintf("magician_card_next_%d_%s_%s_%s_%s", currentPage, selectorID, userID, targetUserID, guildID),
						Style:    discordgo.PrimaryButton,
						Disabled: currentPage == len(paginatedOptions)-1,
					},
					discordgo.Button{
						Label:    "Cancel",
						CustomID: fmt.Sprintf("magician_card_cancel_%s_%s_%s_%s", selectorID, userID, targetUserID, guildID),
						Style:    discordgo.DangerButton,
					},
				},
			})
		} else {
			components = append(components, discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Cancel",
						CustomID: fmt.Sprintf("magician_card_cancel_%s_%s_%s_%s", selectorID, userID, targetUserID, guildID),
						Style:    discordgo.DangerButton,
					},
				},
			})
		}

		content := fmt.Sprintf("🎴 The Magician! Select a card to borrow from %s (Mythic cards are excluded):", targetMention)
		var flags discordgo.MessageFlags
		if len(paginatedOptions) > 1 {
			content += fmt.Sprintf("\n\n⚠️ Only <@%s> can select a card.", userID)
			flags = 0
		} else {
			flags = discordgo.MessageFlagsEphemeral
		}

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    content,
				Components: components,
				Flags:      flags,
			},
		})
	})
}
