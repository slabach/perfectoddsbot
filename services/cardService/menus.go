package cardService

import (
	"fmt"
	"math"
	"perfectOddsBot/models"
	"perfectOddsBot/services/common"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

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

func ShowPointRangeUserSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, cardID uint, cardName string, cardDescription string, userID string, guildID string, db *gorm.DB, maxPointDifference float64) {
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
				Content: fmt.Sprintf("🎴 <@%s> drew **%s**!\n%s\n\nNo users found within %.0f points of you (you have %.1f points). This card fizzles out.", userID, cardName, cardDescription, maxPointDifference, drawer.Points),
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

func ShowTransferPortalUserSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, cardID uint, cardName string, cardDescription string, userID string, guildID string, db *gorm.DB) {
	var drawer models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userID, guildID).First(&drawer).Error; err != nil {
		common.SendError(s, i, err, db)
		return
	}

	hasTradeable, err := HasTradeableCard(db, drawer.ID, guildID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	if !hasTradeable {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("🎴 <@%s> drew **%s**!\n%s\n\nYou have no tradeable cards in your inventory. This card fizzles out.", userID, cardName, cardDescription),
			},
		})
		return
	}

	eligibleUsers, err := GetEligibleUsersWithTradeableCards(db, guildID, userID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	if len(eligibleUsers) == 0 {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("🎴 <@%s> drew **%s**!\n%s\n\nNo other players have tradeable cards. This card fizzles out.", userID, cardName, cardDescription),
			},
		})
		return
	}

	var selectOptions []discordgo.SelectMenuOption
	for _, u := range eligibleUsers {
		displayName := ""
		if u.Username != nil && *u.Username != "" {
			displayName = *u.Username
		} else {
			displayName = fmt.Sprintf("User %s", u.DiscordID)
		}
		if len(displayName) > 100 {
			displayName = displayName[:97] + "..."
		}
		description := fmt.Sprintf("%.1f points", u.Points)
		if len(description) > 100 {
			description = description[:97] + "..."
		}
		value := u.DiscordID
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
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("🎴 <@%s> drew **%s**!\n%s\n\nSelect a user who has a tradeable card to swap with:", userID, cardName, cardDescription),
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
