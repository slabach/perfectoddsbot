package interactionService

import (
	"fmt"
	"perfectOddsBot/models"
	cardService "perfectOddsBot/services/cardService"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

// HandleCardUserSelection handles user selection for cards that require a target (e.g., Pickpocket)
func HandleCardUserSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	customID := i.MessageComponentData().CustomID

	// Parse custom ID: card_<cardID>_select_<userID>_<guildID>
	parts := strings.Split(customID, "_")
	if len(parts) != 5 {
		return fmt.Errorf("invalid card selection custom ID format")
	}

	cardID, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}

	userID := parts[3]
	guildID := parts[4]

	// Verify the interaction user matches the user who drew the card
	if i.Member.User.ID != userID {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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

	// Get selected user
	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no user selected")
	}
	targetUserID := i.MessageComponentData().Values[0]

	// Don't allow targeting yourself
	if targetUserID == userID {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
		return handlePettyTheftSelection(s, i, db, userID, targetUserID, guildID)
	case cards.JesterCardID:
		return handleJesterSelection(s, i, db, userID, targetUserID, guildID)
	default:
		return fmt.Errorf("card %d does not support user selection", cardID)
	}
}

func handlePettyTheftSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	// Execute the steal
	result, err := cards.ExecutePickpocketSteal(db, userID, targetUserID, guildID)
	if err != nil {
		return err
	}

	// Get guild info for pool balance
	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}

	// Get user info
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return err
	}

	// Get target username
	targetUsername := common.GetUsernameWithDB(db, s, guildID, targetUserID)

	// Get card info
	card := cardService.GetCardByID(4)
	if card == nil {
		return fmt.Errorf("card not found")
	}

	// Build embed
	embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

	// Acknowledge interaction and send result
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func handleJesterSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, targetUserID string, guildID string) error {
	// Execute the mute
	result, err := cards.ExecuteJesterMute(s, db, userID, targetUserID, guildID)
	if err != nil {
		return err
	}

	// Get guild info for pool balance
	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		return err
	}

	// Get user info
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&user).Error; err != nil {
		return err
	}

	// Get target username
	targetUsername := common.GetUsernameWithDB(db, s, guildID, targetUserID)

	// Get card info
	card := cardService.GetCardByID(29) // ID 29 is Jester
	if card == nil {
		return fmt.Errorf("card not found")
	}

	// Build embed
	embed := buildCardResultEmbed(card, result, user, targetUsername, guild.Pool)

	// Acknowledge interaction and send result
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	return err
}

func buildCardResultEmbed(card *models.Card, result *models.CardResult, user models.User, targetUsername string, poolBalance float64) *discordgo.MessageEmbed {
	// Determine rarity color
	var color int
	switch card.Rarity {
	case "Common":
		color = cards.C_Common // Gray
	case "Uncommon":
		color = cards.C_Uncommon // Green
	case "Rare":
		color = cards.C_Rare // Blue
	case "Epic":
		color = cards.C_Epic // Purple
	case "Mythic":
		color = cards.C_Mythic // Gold
	default:
		color = cards.C_Common // Gray
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("🎴 Card Effect: %s", card.Name),
		Description: result.Message,
		Color:       color,
		Fields:      []*discordgo.MessageEmbedField{},
	}

	// Add points delta for user
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

	// Add target user points delta if applicable
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
