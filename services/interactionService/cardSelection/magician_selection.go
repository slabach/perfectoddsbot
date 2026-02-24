package cardSelection

import (
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"perfectOddsBot/models"
	cardService "perfectOddsBot/services/cardService"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func MagicianSelectorID() string {
	b := make([]byte, 4)
	_, _ = crand.Read(b)
	return hex.EncodeToString(b)
}

func HandleMagicianCardSelection(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
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

	if !cardService.TryMarkSelectorUsed(customID) {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "❌ This selection has already been used.",
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
				UserID:   drawerUser.ID,
				GuildID:  guildID,
				CardID:   card.ID,
				CardCode: card.Code,
			}
			if err := tx.Create(&inventory).Error; err != nil {
				return err
			}
		}

		targetUsername := common.GetUsernameWithDB(tx, s, guildID, targetUserID)
		embed := BuildCardResultEmbed(card, cardResult, drawerUser, targetUsername, guild.Pool)
		embed.Description = fmt.Sprintf("🎴 The Magician (**<@%s>**) borrowed **%s** from <@%s>!\n\n%s", drawerUser.DiscordID, card.Name, targetUserID, cardResult.Message)

		if cardResult.RequiresSelection {
			if cardResult.SelectionType == "user" {
				if card.ID == cards.HostileTakeoverCardID || card.ID == cards.JusticeCardID {
					cardService.ShowPointRangeUserSelectMenu(s, i, card.ID, card.Name, card.Description, drawerUserID, guildID, tx, 500.0)
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

		if len(paginatedOptions) == 0 {
			targetMention := "<@" + targetUserID + ">"
			components := []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("magician_card_select_%s_%s_%s_%s", selectorID, userID, targetUserID, guildID),
							Placeholder: fmt.Sprintf("No borrowable cards from %s", targetMention),
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     []discordgo.SelectMenuOption{},
							Disabled:    true,
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Cancel",
							CustomID: fmt.Sprintf("magician_card_cancel_%s_%s_%s_%s", selectorID, userID, targetUserID, guildID),
							Style:    discordgo.DangerButton,
						},
					},
				},
			}

			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content:    fmt.Sprintf("🎴 The Magician! There are no borrowable cards from %s (Mythic cards are excluded).", targetMention),
					Components: components,
				},
			})
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
