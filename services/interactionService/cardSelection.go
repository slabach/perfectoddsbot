package interactionService

import (
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"perfectOddsBot/models"
	cardService "perfectOddsBot/services/cardService"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
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
		err = handlePettyTheftSelection(s, i, db, userID, targetUserID, guildID)
	case cards.JesterCardID:
		err = handleJesterSelection(s, i, db, userID, targetUserID, guildID)
	case cards.BetFreezeCardID:
		err = handleBetFreezeSelection(s, i, db, userID, targetUserID, guildID)
	case cards.GrandLarcenyCardID:
		err = handleGrandLarcenySelection(s, i, db, userID, targetUserID, guildID)
	case cards.AntiAntiBetCardID:
		err = handleAntiAntiBetSelection(s, i, db, userID, targetUserID, guildID)
	case cards.HostileTakeoverCardID:
		err = handleHostileTakeoverSelection(s, i, db, userID, targetUserID, guildID)
	case cards.JusticeCardID:
		err = handleJusticeSelection(s, i, db, userID, targetUserID, guildID)
	case cards.TheGossipCardID:
		err = handleTheGossipSelection(s, i, db, userID, targetUserID, guildID)
	case cards.DuelCardID:
		err = handleDuelSelection(s, i, db, userID, targetUserID, guildID)
	case cards.TagCardID:
		err = handleTagSelection(s, i, db, userID, targetUserID, guildID)
	case cards.BountyHunterCardID:
		err = handleBountyHunterSelection(s, i, db, userID, targetUserID, guildID)
	case cards.SocialDistancingCardID:
		err = handleSocialDistancingSelection(s, i, db, userID, targetUserID, guildID)
	case cards.TheLoversCardID:
		err = handleTheLoversSelection(s, i, db, userID, targetUserID, guildID)
	case cards.TheHighPriestessCardID:
		err = handleTheHighPriestessSelection(s, i, db, userID, targetUserID, guildID)
	case cards.TheMagicianCardID:
		err = handleTheMagicianSelection(s, i, db, userID, targetUserID, guildID)
	default:
		cardService.UnmarkSelectorUsed(customID)
		return fmt.Errorf("card %d does not support user selection", cardID)
	}

	if err != nil {
		cardService.UnmarkSelectorUsed(customID)
	}

	return err
}

func handlePettyTheftSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func handleJesterSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func handleBetFreezeSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func handleGrandLarcenySelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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
			result.Message = fmt.Sprintf("Grand Larceny successful! You stole %.0f points from %s!", result.PointsDelta, targetName)
		}

		embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func handleTheGossipSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func handleDuelSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func handleTagSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func handleBountyHunterSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func handleSocialDistancingSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func handleAntiAntiBetSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		embed := buildCardResultEmbed(card, &models.CardResult{
			Message:     fmt.Sprintf("Anti-Anti-Bet active! You bet %.0f points that <@%s> will lose their next bet. If they lose, you'll get %.0f points at even odds (+100).", betAmount, targetUserID, betAmount*2),
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

func handleHostileTakeoverSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		drawerOriginalPoints := drawer.Points
		targetOriginalPoints := target.Points

		drawer.Points, target.Points = target.Points, drawer.Points

		if err := tx.Save(&drawer).Error; err != nil {
			return err
		}
		if err := tx.Save(&target).Error; err != nil {
			return err
		}

		guild, err := guildService.GetGuildInfo(s, tx, guildID, i.ChannelID)
		if err != nil {
			return err
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)

		card := cardService.GetCardByID(cards.HostileTakeoverCardID)
		if card == nil {
			return fmt.Errorf("card not found")
		}

		embed := buildCardResultEmbed(card, &models.CardResult{
			Message:           fmt.Sprintf("Hostile Takeover successful! You swapped points with %s.", targetUsername),
			PointsDelta:       targetOriginalPoints - drawerOriginalPoints,
			PoolDelta:         0,
			TargetUserID:      &targetUserID,
			TargetPointsDelta: drawerOriginalPoints - targetOriginalPoints,
		}, drawer, targetUsername, guild.Pool)

		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "You",
				Value:  fmt.Sprintf("<@%s>: %.1f → %.1f points", drawer.DiscordID, drawerOriginalPoints, drawer.Points),
				Inline: true,
			},
			{
				Name:   "Target",
				Value:  fmt.Sprintf("<@%s>: %.1f → %.1f points", target.DiscordID, targetOriginalPoints, target.Points),
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

func handleJusticeSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func handleTheLoversSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

		embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
			},
		})
	})
}

func handleTheHighPriestessSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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

func handleTheMagicianSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
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
					Content: fmt.Sprintf("The Magician tried to borrow from %s, but they have no eligible cards (Mythic cards are excluded). The card fizzles out.", targetMention),
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
					Content: fmt.Sprintf("The Magician tried to borrow from %s, but they have no eligible cards. The card fizzles out.", targetMention),
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

		selectorID := magicianSelectorID()
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

func magicianSelectorID() string {
	b := make([]byte, 4)
	_, _ = crand.Read(b)
	return hex.EncodeToString(b)
}

func HandleMagicianCardSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	if !cardService.TryMarkSelectorUsed(customID) {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ This selection has already been used.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	parts := strings.Split(customID, "_")
	var drawerUserID, targetUserID, guildID string
	if len(parts) == 7 {
		drawerUserID = parts[4]
		targetUserID = parts[5]
		guildID = parts[6]
	} else if len(parts) == 6 {
		drawerUserID = parts[3]
		targetUserID = parts[4]
		guildID = parts[5]
	} else {
		return fmt.Errorf("invalid magician card selection custom ID format")
	}

	if i.Member.User.ID != drawerUserID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only select a card for your own Magician card.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no card selected")
	}

	selectedValue := i.MessageComponentData().Values[0]
	valueParts := strings.Split(selectedValue, "_")
	if len(valueParts) != 5 {
		return fmt.Errorf("invalid selected value format")
	}

	valueDrawerUserID := valueParts[2]
	if valueDrawerUserID != drawerUserID || valueDrawerUserID != i.Member.User.ID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only select a card for your own Magician card.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	valueTargetUserID := valueParts[3]
	valueGuildID := valueParts[4]
	if valueTargetUserID != targetUserID || valueGuildID != guildID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Invalid card selection. Please try again.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	inventoryID, err := strconv.ParseUint(valueParts[0], 10, 32)
	if err != nil {
		cardService.UnmarkSelectorUsed(customID)
		return fmt.Errorf("invalid inventory ID: %v", err)
	}

	cardID, err := strconv.ParseUint(valueParts[1], 10, 32)
	if err != nil {
		cardService.UnmarkSelectorUsed(customID)
		return fmt.Errorf("invalid card ID: %v", err)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		var inventoryItem models.UserInventory
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = (SELECT id FROM users WHERE discord_id = ? AND guild_id = ?)", inventoryID, targetUserID, guildID).
			First(&inventoryItem).Error; err != nil {
			return fmt.Errorf("inventory item not found: %v", err)
		}

		if inventoryItem.CardID != uint(cardID) {
			return fmt.Errorf("card ID mismatch")
		}

		card := cardService.GetCardByID(uint(cardID))
		if card == nil {
			return fmt.Errorf("card definition not found")
		}

		if card.CardRarity.ID != 0 && card.CardRarity.Name == "Mythic" {
			return fmt.Errorf("cannot borrow Mythic cards")
		}

		var drawerUser models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", drawerUserID, guildID).
			First(&drawerUser).Error; err != nil {
			return err
		}

		var guild models.Guild
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			First(&guild).Error; err != nil {
			return err
		}

		if err := tx.Delete(&inventoryItem).Error; err != nil {
			return err
		}

		cardResult, err := card.Handler(s, tx, drawerUserID, guildID)
		if err != nil {
			return fmt.Errorf("error executing borrowed card: %v", err)
		}

		drawerUser.Points += cardResult.PointsDelta
		if drawerUser.Points < 0 {
			drawerUser.Points = 0
		}
		guild.Pool += cardResult.PoolDelta

		if err := tx.Save(&drawerUser).Error; err != nil {
			return err
		}
		if err := tx.Save(&guild).Error; err != nil {
			return err
		}

		if cardResult.TargetUserID != nil && cardResult.TargetPointsDelta != 0 {
			var targetUser models.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("discord_id = ? AND guild_id = ?", *cardResult.TargetUserID, guildID).
				First(&targetUser).Error; err == nil {
				targetUser.Points += cardResult.TargetPointsDelta
				if targetUser.Points < 0 {
					targetUser.Points = 0
				}
				if err := tx.Save(&targetUser).Error; err != nil {
					return err
				}
			}
		}

		if card.AddToInventory {
			inventory := models.UserInventory{
				UserID:  drawerUser.ID,
				GuildID: guildID,
				CardID:  card.ID,
			}
			if err := tx.Create(&inventory).Error; err != nil {
				return err
			}
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)
		embed := buildCardResultEmbed(card, cardResult, drawerUser, targetUsername, guild.Pool)
		embed.Description = fmt.Sprintf("🎴 The Magician borrowed **%s** from %s!\n\n%s", card.Name, targetUsername, cardResult.Message)

		if cardResult.RequiresSelection {
			if cardResult.SelectionType == "user" {
				if card.ID == cards.HostileTakeoverCardID || card.ID == cards.JusticeCardID {
					cardService.ShowFilteredUserSelectMenu(s, i, card.ID, card.Name, card.Description, drawerUserID, guildID, tx, 500.0)
				} else {
					cardService.ShowUserSelectMenu(s, i, card.ID, card.Name, card.Description, drawerUserID, guildID, db)
				}
				return nil
			} else if cardResult.SelectionType == "bet" {
				cardService.ShowBetSelectMenu(s, i, card.ID, card.Name, card.Description, drawerUserID, guildID, db)
				return nil
			}
		}

		if len(card.Options) > 0 {
			cardService.ShowCardOptionsMenu(s, i, card.ID, card.Name, card.Description, drawerUserID, guildID, db, card.Options)
			return nil
		}

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

func HandleMagicianCardPagination(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	parts := strings.Split(customID, "_")
	if len(parts) != 8 {
		return fmt.Errorf("invalid pagination custom ID format")
	}

	direction := parts[2]
	currentPage, err := strconv.Atoi(parts[3])
	if err != nil {
		return fmt.Errorf("invalid page number: %v", err)
	}

	selectorID := parts[4]
	userID := parts[5]
	targetUserID := parts[6]
	guildID := parts[7]

	if i.Member.User.ID != userID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only paginate your own Magician card selection.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var targetUser models.User
		if err := tx.Where("discord_id = ? AND guild_id = ?", targetUserID, guildID).First(&targetUser).Error; err != nil {
			return err
		}

		var inventory []models.UserInventory
		if err := tx.Where("user_id = ? AND guild_id = ? AND deleted_at IS NULL", targetUser.ID, guildID).Find(&inventory).Error; err != nil {
			return err
		}

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

		var paginatedOptions [][]discordgo.SelectMenuOption
		minValues := 1
		for i := 0; i < len(selectOptions); i += 25 {
			end := i + 25
			if end > len(selectOptions) {
				end = len(selectOptions)
			}
			paginatedOptions = append(paginatedOptions, selectOptions[i:end])
		}

		newPage := currentPage
		if direction == "next" {
			newPage++
		} else if direction == "prev" {
			newPage--
		}

		if newPage < 0 {
			newPage = 0
		}
		if newPage >= len(paginatedOptions) {
			newPage = len(paginatedOptions) - 1
		}

		targetMention := "<@" + targetUserID + ">"
		components := []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						MenuType:    discordgo.StringSelectMenu,
						CustomID:    fmt.Sprintf("magician_card_select_%s_%s_%s_%s", selectorID, userID, targetUserID, guildID),
						Placeholder: fmt.Sprintf("Select a card to borrow from %s", targetMention),
						MinValues:   &minValues,
						MaxValues:   1,
						Options:     paginatedOptions[newPage],
					},
				},
			},
		}

		if len(paginatedOptions) > 1 {
			components = append(components, discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Previous",
						CustomID: fmt.Sprintf("magician_card_prev_%d_%s_%s_%s_%s", newPage, selectorID, userID, targetUserID, guildID),
						Style:    discordgo.PrimaryButton,
						Disabled: newPage == 0,
					},
					discordgo.Button{
						Label:    "Next",
						CustomID: fmt.Sprintf("magician_card_next_%d_%s_%s_%s_%s", newPage, selectorID, userID, targetUserID, guildID),
						Style:    discordgo.PrimaryButton,
						Disabled: newPage == len(paginatedOptions)-1,
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
		if len(paginatedOptions) > 1 {
			content += fmt.Sprintf("\n\n⚠️ Only <@%s> can select a card.", userID)
		}

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    content,
				Components: components,
			},
		})
	})
}

func HandleMagicianCardCancel(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	parts := strings.Split(customID, "_")
	var userID string
	if len(parts) == 7 {
		userID = parts[5]
	} else if len(parts) == 6 {
		userID = parts[3]
	} else {
		return fmt.Errorf("invalid cancel custom ID format")
	}

	if i.Member.User.ID != userID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only cancel your own Magician card selection.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    "❌ The Magician card selection was cancelled.",
			Components: []discordgo.MessageComponent{},
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
}

func buildCardResultEmbed(card *models.Card, result *models.CardResult, user models.User, targetUsername string, poolBalance float64) *discordgo.MessageEmbed {
	var color int
	if card.CardRarity.ID != 0 {
		color = cardService.ParseHexColor(card.CardRarity.Color)
	} else {
		color = 0x95A5A6
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎴 Card Effect: %s", card.Name),
		Description: result.Message,
		Color:       color,
		Fields:      []*discordgo.MessageEmbedField{},
	}

	if result.PointsDelta != 0 {
		sign := "+"
		if result.PointsDelta < 0 {
			sign = ""
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "You",
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
			Name:   "Target",
			Value:  fmt.Sprintf("<@%s>: %s%.1f points", *result.TargetUserID, sign, result.TargetPointsDelta),
			Inline: true,
		})
	}

	return embed
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

		embed := buildCardResultEmbed(card, &models.CardResult{
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

			if selectedOptionID == 1 {
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
			} else if selectedOptionID == 2 {
				var allUsers []models.User
				if err := tx.Where("guild_id = ? AND deleted_at IS NULL", guildID).Find(&allUsers).Error; err != nil {
					return err
				}

				messageDetails := ""

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
				}

				result = &models.CardResult{
					Message:     fmt.Sprintf("<@%s> chose **Chaos**! Every player randomly gained or lost 1-500 points:\n\n%s", user.DiscordID, messageDetails),
					PointsDelta: 0,
					PoolDelta:   0,
				}
			} else {
				return fmt.Errorf("invalid option ID for The Wheel of Fortune")
			}

			embed := buildCardResultEmbed(card, result, user, "", guild.Pool)
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{embed},
				},
			})
		}

		if selectedOptionID == 2 {
			var inventory models.UserInventory
			result := tx.Where("user_id = ? AND guild_id = ? AND card_id = ?", user.ID, guildID, cardID).First(&inventory)
			if result.Error == nil {
				if err := tx.Delete(&inventory).Error; err != nil {
					return fmt.Errorf("error removing card from inventory: %v", err)
				}
			}

			embed := buildCardResultEmbed(card, &models.CardResult{
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

		embed := buildCardResultEmbed(card, &models.CardResult{
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
		showBetSelectMenuForPlayCard(s, i, db, selectedCardID, userID, guildID)
		return nil
	}

	if selectedCardID == cards.PoolBoyCardID {
		return handlePoolBoyPlay(s, i, db, selectedCardID, userID, guildID)
	}

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "This card type is not yet supported for manual play.",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func showBetSelectMenuForPlayCard(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, cardID uint, userID string, guildID string) {
	var results []struct {
		BetID       uint
		Description string
		Option1     string
		Option2     string
		TotalAmount int
	}

	err := db.Table("bet_entries").
		Select("bets.id as bet_id, bets.description, bets.option1, bets.option2, SUM(bet_entries.amount) as total_amount").
		Joins("JOIN bets ON bets.id = bet_entries.bet_id").
		Where("bet_entries.user_id = (SELECT id FROM users WHERE discord_id = ? AND guild_id = ?) AND bets.paid = 0 AND bets.guild_id = ? AND bet_entries.deleted_at IS NULL", userID, guildID, guildID).
		Group("bets.id, bets.description, bets.option1, bets.option2").
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
				Content: "You don't have any active bets to cancel.",
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
		label := fmt.Sprintf("%s (%d pts)", res.Description, res.TotalAmount)
		if len(label) > 100 {
			label = label[:97] + "..."
		}

		options = append(options, discordgo.SelectMenuOption{
			Label: label,
			Value: fmt.Sprintf("%d", res.BetID),
		})
	}

	card := cardService.GetCardByID(cardID)
	cardName := "Card"
	if card != nil {
		cardName = card.Name
	}

	minValues := 1
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 Playing **%s**\n\nSelect an active bet to cancel:", cardName),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("playcard_bet_%d_%s_%s", cardID, userID, guildID),
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

		embed := buildCardResultEmbed(card, &models.CardResult{
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

func handlePoolBoyPlay(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, cardID uint, userID string, guildID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&user).Error; err != nil {
			return fmt.Errorf("user not found: %v", err)
		}

		var guild models.Guild
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			First(&guild).Error; err != nil {
			return fmt.Errorf("guild not found: %v", err)
		}

		card := cardService.GetCardByID(cardID)
		if card == nil {
			return fmt.Errorf("card not found: %d", cardID)
		}

		guild.PoolDrainUntil = nil
		if err := tx.Save(&guild).Error; err != nil {
			return fmt.Errorf("error clearing pool drain: %v", err)
		}

		if err := cardService.PlayCardFromInventoryWithMessage(s, tx, user, cardID, fmt.Sprintf("<@%s> played **%s** and cleaned the algae from the pool! The pool drain effect has been stopped.", userID, card.Name)); err != nil {
			return fmt.Errorf("error consuming card: %v", err)
		}

		embed := buildCardResultEmbed(card, &models.CardResult{
			Message:     "You cleaned the algae from the pool! The algae bloom effect has been stopped.",
			PointsDelta: 0,
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
