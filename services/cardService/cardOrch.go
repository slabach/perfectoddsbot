package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

// DrawCard handles the /draw-card command
func DrawCard(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	// Get guild info
	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	// Get or create user
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

	// Update username
	username := common.GetUsernameFromUser(i.Member.User)
	common.UpdateUserUsername(db, &user, username)

	// Get card draw settings from guild
	drawCardCost := guild.CardDrawCost
	cardDrawCooldown := time.Duration(guild.CardDrawCooldownMinutes) * time.Minute

	// Check cooldown
	if user.LastCardDraw != nil {
		timeSinceLastDraw := time.Since(*user.LastCardDraw)
		if timeSinceLastDraw < cardDrawCooldown {
			remainingTime := cardDrawCooldown - timeSinceLastDraw
			minutes := int(remainingTime.Minutes())
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("You must wait %d more minutes before drawing another card.", minutes+1),
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				common.SendError(s, i, err, db)
			}
			return
		}
	}

	// Check if user has enough points
	if user.Points < drawCardCost {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("You need at least %.0f points to draw a card. You have %.1f points.", drawCardCost, user.Points),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	// Start transaction for atomic operations
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Deduct cost
	user.Points -= drawCardCost

	// Add to pool
	guild.Pool += drawCardCost

	// Update last card draw timestamp
	now := time.Now()
	user.LastCardDraw = &now

	// Pick random card
	card := PickRandomCard()
	if card == nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("no cards available"), db)
		return
	}

	// Execute card handler
	cardResult, err := card.Handler(s, db, userID, guildID)
	if err != nil {
		tx.Rollback()
		common.SendError(s, i, fmt.Errorf("error executing card effect: %v", err), db)
		return
	}

	// If card requires user selection (e.g., Pickpocket), handle it specially
	if cardResult.RequiresSelection {
		// Save intermediate state - we need to store the card ID and user state
		// For now, we'll handle Pickpocket selection immediately
		if cardResult.SelectionType == "user" {
			// Show user select menu
			showUserSelectMenu(s, i, card.ID, card.Name, card.Description, userID, guildID, db)

			// Still deduct the draw cost and update pool
			user.Points += cardResult.PointsDelta
			guild.Pool += cardResult.PoolDelta

			// Save partial state (cost deducted, pool updated, cooldown set)
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

	// Apply card effects
	user.Points += cardResult.PointsDelta
	guild.Pool += cardResult.PoolDelta

	// Update target user if applicable
	var targetUsername string
	if cardResult.TargetUserID != nil {
		var targetUser models.User
		if err := tx.Where("discord_id = ? AND guild_id = ?", *cardResult.TargetUserID, guildID).First(&targetUser).Error; err == nil {
			targetUser.Points += cardResult.TargetPointsDelta
			if targetUser.Points < 0 {
				targetUser.Points = 0
			}
			tx.Save(&targetUser)
			targetUsername = common.GetUsernameWithDB(db, s, guildID, *cardResult.TargetUserID)
		}
	}

	// Save all changes
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

	// Build embed response
	embed := buildCardEmbed(card, cardResult, user, targetUsername, guild.Pool, drawCardCost)

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

// showUserSelectMenu displays a user select menu for cards that require target selection
func showUserSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, cardID int, cardName string, cardDescription string, userID string, guildID string, db *gorm.DB) {
	minValues := 1
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 You drew **%s**!\n%s\n\nSelect a user to target:", cardName, cardDescription),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.UserSelectMenu,
							CustomID:    fmt.Sprintf("card_%d_select_%s_%s", cardID, userID, guildID),
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

// buildCardEmbed creates a rich embed for the card draw result
func buildCardEmbed(card *models.Card, result *models.CardResult, user models.User, targetUsername string, poolBalance float64, drawCardCost float64) *discordgo.MessageEmbed {
	// Determine rarity color
	var color int
	switch card.Rarity {
	case "Common":
		color = 0x95A5A6 // Gray
	case "Rare":
		color = 0x3498DB // Blue
	case "Epic":
		color = 0x9B59B6 // Purple
	case "Mythic":
		color = 0xF1C40F // Gold
	default:
		color = 0x95A5A6
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎴 You Drew: %s", card.Name),
		Description: card.Description,
		Color:       color,
		Fields:      []*discordgo.MessageEmbedField{},
	}

	// Add rarity field
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Rarity",
		Value:  card.Rarity,
		Inline: true,
	})

	// Add result message
	if result.Message != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Effect",
			Value:  result.Message,
			Inline: false,
		})
	}

	// Add points delta for user
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

	// Add target user points delta if applicable
	if result.TargetUserID != nil && result.TargetPointsDelta != 0 {
		sign := "+"
		if result.TargetPointsDelta < 0 {
			sign = ""
		}
		// We need to calculate the target's new total (this will be passed in if needed)
		// For now, just show the change
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Target Change",
			Value:  fmt.Sprintf("<@%s>: %s%.1f points", *result.TargetUserID, sign, result.TargetPointsDelta),
			Inline: true,
		})
	}

	// Add pool balance
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Pool Balance",
		Value:  fmt.Sprintf("%.1f points", poolBalance),
		Inline: true,
	})

	// Add cost info
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Cost: -%.0f points | Added %.0f to pool", drawCardCost, drawCardCost),
	}

	return embed
}
