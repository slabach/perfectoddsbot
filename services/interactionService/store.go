package interactionService

import (
	"fmt"
	"perfectOddsBot/models"
	cardService "perfectOddsBot/services/cardService"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func HandleStoreSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	parts := strings.Split(customID, "_")
	if len(parts) != 4 {
		return fmt.Errorf("invalid store selection custom ID format")
	}

	userID := parts[2]
	guildID := parts[3]

	if i.Member.User.ID != userID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only select cards for your own purchase.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no card selected")
	}

	cardIDStr := i.MessageComponentData().Values[0]
	cardID, err := strconv.ParseUint(cardIDStr, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}

	// Get card info from database to get store cost
	var dbCard models.Card
	err = db.Where("id = ? AND store_cost IS NOT NULL AND active = ?", cardID, true).First(&dbCard).Error
	if err != nil {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Card not found or not available for purchase. Please refresh the store.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	// Get card from registry for name/description
	card := cardService.GetCardByID(uint(cardID))
	if card == nil {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ Card not found in registry. Please refresh the store.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	cost := 0.0
	if dbCard.StoreCost != nil {
		cost = *dbCard.StoreCost
	}

	// Update the message to show selected card and update purchase button
	originalMessage := i.Message
	if originalMessage == nil {
		return fmt.Errorf("original message not found")
	}

	// Find the purchase button and update its customID to include card ID
	var updatedComponents []discordgo.MessageComponent
	for _, row := range originalMessage.Components {
		var rowComponents []discordgo.MessageComponent
		switch r := row.(type) {
		case *discordgo.ActionsRow:
			rowComponents = r.Components
		case discordgo.ActionsRow:
			rowComponents = r.Components
		default:
			updatedComponents = append(updatedComponents, row)
			continue
		}

		var updatedRowComponents []discordgo.MessageComponent
		for _, comp := range rowComponents {
			var btn *discordgo.Button
			switch c := comp.(type) {
			case *discordgo.Button:
				btn = c
			case discordgo.Button:
				btn = &c
			default:
				updatedRowComponents = append(updatedRowComponents, comp)
				continue
			}
			if strings.HasPrefix(btn.CustomID, "store_purchase_") {
				newButton := *btn
				newButton.CustomID = fmt.Sprintf("store_purchase_%s_%s_%d", userID, guildID, cardID)
				newButton.Label = fmt.Sprintf("Purchase %s (%.0f points)", card.Name, cost)
				updatedRowComponents = append(updatedRowComponents, &newButton)
			} else {
				updatedRowComponents = append(updatedRowComponents, comp)
			}
		}
		updatedComponents = append(updatedComponents, &discordgo.ActionsRow{
			Components: updatedRowComponents,
		})
	}

	// Update embed to show selected card
	var updatedEmbeds []*discordgo.MessageEmbed
	if len(originalMessage.Embeds) > 0 {
		originalEmbed := originalMessage.Embeds[0]
		updatedEmbed := *originalEmbed
		updatedEmbed.Description = fmt.Sprintf("Purchase specific cards directly from the store. Points spent go into the pool.\n\n**Selected: %s** (%.0f points)", card.Name, cost)
		updatedEmbeds = append(updatedEmbeds, &updatedEmbed)
	}

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     updatedEmbeds,
			Components: updatedComponents,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
}

func HandleStorePurchase(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	parts := strings.Split(customID, "_")
	if len(parts) < 5 {
		return fmt.Errorf("invalid store purchase custom ID format")
	}

	userID := parts[2]
	guildID := parts[3]
	cardIDStr := parts[4]

	if i.Member.User.ID != userID {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You can only purchase cards for yourself.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}

	cardID, err := strconv.ParseUint(cardIDStr, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid card ID: %v", err)
	}

	return cardService.ProcessStorePurchase(s, i, db, uint(cardID), userID, guildID)
}
