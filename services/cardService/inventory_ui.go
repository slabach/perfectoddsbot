package cardService

import (
	"fmt"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"time"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func MyInventory(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
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

	inventory, err := getUserInventory(db, user.ID, guildID)
	if err != nil {
		common.SendError(s, i, fmt.Errorf("error fetching inventory: %v", err), db)
		return
	}

	cardCounts := make(map[uint]int)
	for _, item := range inventory {
		cardCounts[item.CardID]++
	}

	now := time.Now()
	expirationTime := now.Add(-12 * time.Hour)
	hasShoppingSpree := false
	for _, item := range inventory {
		if item.CardID == cards.ShoppingSpreeCardID {
			if item.CreatedAt.After(expirationTime) || item.CreatedAt.Equal(expirationTime) {
				hasShoppingSpree = true
				break
			}
		}
	}

	resetPeriod := time.Duration(guild.CardDrawCooldownMinutes) * time.Minute

	var countdownText string
	if user.FirstCardDrawCycle != nil {
		resetTime := user.FirstCardDrawCycle.Add(resetPeriod)
		if now.Before(resetTime) {
			timeRemaining := resetTime.Sub(now)
			hours := int(timeRemaining.Hours())
			minutes := int(timeRemaining.Minutes()) % 60
			seconds := int(timeRemaining.Seconds()) % 60
			if hours > 0 {
				countdownText = fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
			} else if minutes > 0 {
				countdownText = fmt.Sprintf("%dm %ds", minutes, seconds)
			} else {
				countdownText = fmt.Sprintf("%ds", seconds)
			}
		} else {
			countdownText = "Next Draw Resets"
		}
	} else {
		countdownText = "No draws yet"
	}

	var nextDrawCount int
	if user.FirstCardDrawCycle != nil {
		resetTime := user.FirstCardDrawCycle.Add(resetPeriod)
		if now.After(resetTime) || now.Equal(resetTime) {
			nextDrawCount = 0
		} else {
			nextDrawCount = user.CardDrawCount
		}
	} else {
		nextDrawCount = 0
	}

	var nextDrawCost float64
	switch nextDrawCount {
	case 0:
		nextDrawCost = guild.CardDrawCost
	case 1:
		nextDrawCost = guild.CardDrawCost * 10
	default:
		nextDrawCost = guild.CardDrawCost * 50
	}

	hasLuckyHorseshoe := cardCounts[cards.LuckyHorseshoeCardID] > 0
	if hasLuckyHorseshoe {
		nextDrawCost = nextDrawCost * 0.5
	}

	hasUnluckyCat := cardCounts[cards.UnluckyCatCardID] > 0
	if hasUnluckyCat {
		nextDrawCost = nextDrawCost * 2.0
	}

	hasCoupon := cardCounts[cards.CouponCardID] > 0
	if hasCoupon {
		nextDrawCost = nextDrawCost * 0.75
	}

	if hasShoppingSpree {
		nextDrawCost = nextDrawCost * 0.5
	}

	var lockoutText string
	if user.CardDrawTimeoutUntil != nil && now.Before(*user.CardDrawTimeoutUntil) {
		timeRemaining := user.CardDrawTimeoutUntil.Sub(now)
		hours := int(timeRemaining.Hours())
		minutes := int(timeRemaining.Minutes()) % 60
		seconds := int(timeRemaining.Seconds()) % 60
		if hours > 0 {
			lockoutText = fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
		} else if minutes > 0 {
			lockoutText = fmt.Sprintf("%dm %ds", minutes, seconds)
		} else {
			lockoutText = fmt.Sprintf("%ds", seconds)
		}
	}

	if len(cardCounts) == 0 {
		fields := []*discordgo.MessageEmbedField{
			{
				Name:   "⏱️ Timer Reset",
				Value:  countdownText,
				Inline: true,
			},
			{
				Name:   "💰 Next Draw Cost",
				Value:  fmt.Sprintf("%.0f points", nextDrawCost),
				Inline: true,
			},
		}

		if lockoutText != "" {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "🚫 Draw Lockout",
				Value:  lockoutText,
				Inline: true,
			})
		}

		embed := &discordgo.MessageEmbed{
			Title:       "🎴 Your Inventory",
			Description: "Your inventory is empty. Draw some cards to add them to your hand!",
			Color:       0x3498DB,
			Fields:      fields,
		}

		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
		}
		return
	}

	rarityOrder := []string{"Mythic", "Epic", "Rare", "Uncommon", "Common"}
	cardsByRarity := make(map[string][]struct {
		Card  *models.Card
		Count int
	})

	for cardID, count := range cardCounts {
		card := GetCardByID(uint(cardID))
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

	embed := &discordgo.MessageEmbed{
		Title:       "🎴 Your Inventory",
		Description: "Cards currently in your hand",
		Color:       0x3498DB,
		Fields:      []*discordgo.MessageEmbedField{},
	}

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
			fieldValue += fmt.Sprintf("**%s**%s\n%s\n\n", cardInfo.Card.Name, quantityText, cardInfo.Card.Description)
		}

		var rarityEmoji string = "🤍" // Default to Common emoji
		if len(cardsHeld) > 0 && cardsHeld[0].Card.CardRarity.ID != 0 {
			rarityEmoji = cardsHeld[0].Card.CardRarity.Icon
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("%s %s", rarityEmoji, rarity),
			Value:  fieldValue,
			Inline: false,
		})
	}

	totalCards := 0
	for _, count := range cardCounts {
		totalCards += count
	}
	embed.Footer = &discordgo.MessageEmbedFooter{
		Text: fmt.Sprintf("Total cards: %d", totalCards),
	}

	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "⏱️ Timer Reset",
		Value:  countdownText,
		Inline: true,
	})

	costText := fmt.Sprintf("%.0f points", nextDrawCost)
	if nextDrawCost == 0 {
		costText = "Free (Generous Donation)"
	} else if hasShoppingSpree {
		costText = fmt.Sprintf("%.0f points (Shopping Spree: -50%%)", nextDrawCost)
	}
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "💰 Next Draw Cost",
		Value:  costText,
		Inline: true,
	})

	if lockoutText != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "🚫 Draw Lockout",
			Value:  lockoutText,
			Inline: true,
		})
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
}
