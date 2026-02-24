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

func PlayCard(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	userID := i.Member.User.ID
	guildID := i.GuildID

	guild, err := guildService.GetGuildInfo(s, db, guildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error getting guild info: %v", err), db)
		return
	}

	var user models.User
	result := db.Where(models.User{DiscordID: userID, GuildID: guildID}).Attrs(models.User{Points: guild.StartingPoints}).FirstOrCreate(&user)
	if result.Error != nil {
		common.SendError(s, i, fmt.Errorf("error fetching user: %v", result.Error), db)
		return
	}

	showPlayableCardSelectMenu(s, i, db, userID, guildID, user.ID)
}

func showPlayableCardSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userID string, guildID string, userDBID uint) {
	inventory, err := getUserInventory(db, userDBID, guildID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error fetching inventory: %v", err), db)
		return
	}

	inventoryMap := make(map[uint]int)
	now := time.Now()
	for _, item := range inventory {
		if item.ExpiresAt != nil && item.ExpiresAt.Before(now) {
			continue
		}
		inventoryMap[item.CardID]++
	}

	var playableCards []struct {
		Card  *models.Card
		Count int
	}

	for cardID, count := range inventoryMap {
		card := GetCardByID(uint(cardID))
		if card != nil && card.UserPlayable {
			playableCards = append(playableCards, struct {
				Card  *models.Card
				Count int
			}{Card: card, Count: count})
		}
	}

	if len(playableCards) == 0 {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You don't have any playable cards in your inventory. Draw some cards to get started!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	maxOptions := 25
	if len(playableCards) > maxOptions {
		playableCards = playableCards[:maxOptions]
	}

	var selectOptions []discordgo.SelectMenuOption
	for _, pc := range playableCards {
		label := pc.Card.Name
		if pc.Count > 1 {
			label = fmt.Sprintf("%s (x%d)", pc.Card.Name, pc.Count)
		}

		if len(label) > 100 {
			label = label[:97] + "..."
		}

		description := pc.Card.Description
		if len(description) > 100 {
			description = description[:97] + "..."
		}

		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label:       label,
			Value:       fmt.Sprintf("%d", pc.Card.ID),
			Description: description,
			Emoji:       nil,
			Default:     false,
		})
	}

	minValues := 1
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Select a card to play from your inventory:",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    fmt.Sprintf("playcard_select_%s_%s", userID, guildID),
							Placeholder: "Choose a card to play...",
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
