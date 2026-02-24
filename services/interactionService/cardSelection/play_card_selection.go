package cardSelection

import (
	"fmt"
	"perfectOddsBot/models"
	cardService "perfectOddsBot/services/cardService"
	"perfectOddsBot/services/common"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ShowBetSelectMenuForPlayCard(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, cardID uint, userID string, guildID string) {
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

func HandleEmperorPlay(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, cardID uint, userID string, guildID string) error {
	card := cardService.GetCardByID(cardID)
	if card == nil {
		return fmt.Errorf("card not found: %d", cardID)
	}

	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %v", err)
	}

	var guild models.Guild
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return fmt.Errorf("guild not found: %v", err)
	}

	embed := BuildCardResultEmbed(card, &models.CardResult{
		Message:     "You gained Authority for 1 hour! 10% of all points won by other players will be diverted to the pool.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, user, "", guild.Pool)

	// Fail fast on Discord errors - respond first
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		return err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		var txUser models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&txUser).Error; err != nil {
			return fmt.Errorf("user not found: %v", err)
		}

		var txGuild models.Guild
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			First(&txGuild).Error; err != nil {
			return fmt.Errorf("guild not found: %v", err)
		}

		oneHourLater := time.Now().Add(1 * time.Hour)
		txGuild.EmperorActiveUntil = &oneHourLater
		txGuild.EmperorHolderDiscordID = &userID
		if err := tx.Save(&txGuild).Error; err != nil {
			return fmt.Errorf("error setting Emperor state: %v", err)
		}

		if err := cardService.PlayCardFromInventoryInTransaction(tx, txUser, cardID); err != nil {
			return fmt.Errorf("error consuming card: %v", err)
		}

		return nil
	}); err != nil {
		return err
	}
	notificationMessage := fmt.Sprintf("<@%s> played **%s** and gained Authority! For the next hour, 10%% of all points won by other players will be diverted to the pool.", userID, card.Name)
	if err := cardService.NotifyCardPlayedWithMessage(s, db, user, card, notificationMessage); err != nil {
		fmt.Printf("Error sending card played notification: %v\n", err)
	}

	return nil
}

func HandlePoolBoyPlay(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, cardID uint, userID string, guildID string) error {
	card := cardService.GetCardByID(cardID)
	if card == nil {
		return fmt.Errorf("card not found: %d", cardID)
	}

	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %v", err)
	}

	var guild models.Guild
	if err := db.Where("guild_id = ?", guildID).First(&guild).Error; err != nil {
		return fmt.Errorf("guild not found: %v", err)
	}

	// Check if a pool drain is active before proceeding
	if guild.PoolDrainUntil == nil {
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "There is no active pool drain to clean. The Pool Boy card can only be used when a pool drain is active.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			return err
		}
		return nil
	}

	embed := BuildCardResultEmbed(card, &models.CardResult{
		Message:     "You cleaned the algae from the pool! The algae bloom effect has been stopped.",
		PointsDelta: 0,
		PoolDelta:   0,
	}, user, "", guild.Pool)

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		return err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		var txUser models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("discord_id = ? AND guild_id = ?", userID, guildID).
			First(&txUser).Error; err != nil {
			return fmt.Errorf("user not found: %v", err)
		}

		var txGuild models.Guild
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ?", guildID).
			First(&txGuild).Error; err != nil {
			return fmt.Errorf("guild not found: %v", err)
		}

		// Check again in transaction (race condition protection)
		if txGuild.PoolDrainUntil == nil {
			// Send followup ephemeral message since we already responded
			if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: "The pool drain was already cleared by another action. Your card was not consumed.",
				Flags:   discordgo.MessageFlagsEphemeral,
			}); err != nil {
				// Log but don't fail the transaction
				fmt.Printf("Error sending followup message: %v\n", err)
			}
			return nil // Return nil to avoid consuming the card
		}

		txGuild.PoolDrainUntil = nil
		if err := tx.Save(&txGuild).Error; err != nil {
			return fmt.Errorf("error clearing pool drain: %v", err)
		}

		if err := cardService.PlayCardFromInventoryInTransaction(tx, txUser, cardID); err != nil {
			return fmt.Errorf("error consuming card: %v", err)
		}

		return nil
	}); err != nil {
		return err
	}

	notificationMessage := fmt.Sprintf("<@%s> played **%s** and cleaned the algae from the pool! The pool drain effect has been stopped.", userID, card.Name)
	if err := cardService.NotifyCardPlayedWithMessage(s, db, user, card, notificationMessage); err != nil {
		fmt.Printf("Error sending card played notification: %v\n", err)
	}

	return nil
}
