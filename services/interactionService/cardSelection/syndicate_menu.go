package cardSelection

import (
	"fmt"
	"strings"
	"time"

	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

type entryChoice struct {
	EntryID       uint
	Option        int
	Amount        int
	Spread        *float64
	BetID         uint
	Description   string
	Option1       string
	Option2       string
	Active        bool
	Paid          bool
	CfbdID        *string
	EspnID        *string
	GameStart     *time.Time
	OpeningSpread *float64
	BetSpread     *float64
}

func (e entryChoice) label() string {
	side := e.Option1
	if e.Option == 2 {
		side = e.Option2
	}
	label := fmt.Sprintf("%s (%s)", e.Description, side)
	if len(label) > 100 {
		label = label[:97] + "..."
	}
	return label
}

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func showEntryMenu(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, userDBID uint, guildID string, discordID string, cardID uint, mode string, rows []entryChoice, prompt string) error {
	if len(rows) == 0 {
		return respondEphemeral(s, i, prompt)
	}
	if len(rows) > 25 {
		rows = rows[:25]
	}
	options := make([]discordgo.SelectMenuOption, 0, len(rows))
	for _, row := range rows {
		options = append(options, discordgo.SelectMenuOption{
			Label: row.label(),
			Value: fmt.Sprintf("%d", row.EntryID),
		})
	}
	minValues := 1
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: prompt,
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						MenuType:    discordgo.StringSelectMenu,
						CustomID:    fmt.Sprintf("synentry_%s_%d_%s_%s", mode, cardID, discordID, guildID),
						Placeholder: "Choose a bet entry...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options:     options,
					},
				}},
			},
		},
	})
}

func loadEntries(db *gorm.DB, userDBID uint, guildID string, where string, args ...interface{}) ([]entryChoice, error) {
	var rows []entryChoice
	q := db.Table("bet_entries").
		Select(`bet_entries.id as entry_id, bet_entries.option, bet_entries.amount, bet_entries.spread,
			bets.id as bet_id, bets.description, bets.option1, bets.option2, bets.active, bets.paid,
			bets.cfbd_id, bets.espn_id, bets.game_start_date as game_start, bets.opening_spread, bets.spread as bet_spread`).
		Joins("JOIN bets ON bets.id = bet_entries.bet_id AND bets.deleted_at IS NULL").
		Where("bet_entries.user_id = ? AND bet_entries.deleted_at IS NULL AND bets.guild_id = ? AND bets.paid = ?", userDBID, guildID, false)
	if where != "" {
		q = q.Where(where, args...)
	}
	err := q.Order("bet_entries.id ASC").Limit(25).Scan(&rows).Error
	return rows, err
}

func activeEntryWhere() (string, []interface{}) {
	return "bets.active = ?", []interface{}{true}
}

func spreadSportsWhere() string {
	return "bets.spread IS NOT NULL AND (bets.cfbd_id IS NOT NULL OR bets.espn_id IS NOT NULL)"
}

// ShowSyndicateEntryMenu lists the entries a played card is allowed to touch.
func ShowSyndicateEntryMenu(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, user models.User, cardID uint) error {
	var where string
	var args []interface{}
	prompt := "Select one of your open bet entries."
	mode := "active"
	switch cardID {
	case cards.PointShaverCardID:
		where = "bets.active = ? AND " + spreadSportsWhere()
		args = []interface{}{true}
		prompt = "Select an open CFB or CBB spread entry to shave."
		mode = "shave"
	case cards.TheFixIsInCardID:
		where = spreadSportsWhere()
		prompt = "Select an unpaid CFB or CBB spread entry. Locked games are included."
		mode = "fix"
	default:
		where, args = activeEntryWhere()
	}
	rows, err := loadEntries(db, user.ID, user.GuildID, where, args...)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return respondEphemeral(s, i, "No eligible bet entries. The card stays in your inventory.")
	}
	return showEntryMenu(s, i, db, user.ID, user.GuildID, user.DiscordID, cardID, mode, rows, prompt)
}

func ShowWiretapChoice(s *discordgo.Session, i *discordgo.InteractionCreate, user models.User) error {
	minValues := 1
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Backroom Wiretap: lock an open entry to the opening line, or refund a locked entry before kickoff.",
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						MenuType:    discordgo.StringSelectMenu,
						CustomID:    fmt.Sprintf("synwire_%d_%s_%s", cards.BackroomWiretapCardID, user.DiscordID, user.GuildID),
						Placeholder: "Choose a move...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options: []discordgo.SelectMenuOption{
							{Label: "Opening line", Value: "open", Description: "Rewrite one open spread entry"},
							{Label: "Refund", Value: "refund", Description: "Refund one locked entry before kickoff"},
						},
					},
				}},
			},
		},
	})
}

func ShowWiretapEntries(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, user models.User, mode string) error {
	var where string
	var args []interface{}
	prompt := "Select an open spread entry."
	if mode == "refund" {
		where = "bets.active = ? AND bets.game_start_date IS NOT NULL AND bets.game_start_date > ?"
		args = []interface{}{false, time.Now()}
		prompt = "Select a locked entry that has not kicked off."
	} else {
		where = "bets.active = ? AND bets.spread IS NOT NULL"
		args = []interface{}{true}
	}
	rows, err := loadEntries(db, user.ID, user.GuildID, where, args...)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return respondEphemeral(s, i, "No eligible entries. The card stays in your inventory.")
	}
	return showEntryMenu(s, i, db, user.ID, user.GuildID, user.DiscordID, cards.BackroomWiretapCardID, mode, rows, prompt)
}

func ShowParlayMenu(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, user models.User) error {
	var parlays []models.Parlay
	if err := db.Preload("ParlayEntries").
		Where("user_id = ? AND guild_id = ? AND status IN ? AND deleted_at IS NULL", user.ID, user.GuildID, []string{"pending", "partial"}).
		Find(&parlays).Error; err != nil {
		return err
	}
	options := make([]discordgo.SelectMenuOption, 0)
	for _, p := range parlays {
		if len(p.ParlayEntries) < 4 {
			continue
		}
		lost := false
		for _, leg := range p.ParlayEntries {
			if leg.Won != nil && !*leg.Won {
				lost = true
			}
		}
		if lost {
			continue
		}
		options = append(options, discordgo.SelectMenuOption{
			Label: fmt.Sprintf("%d-leg parlay (%d pts)", len(p.ParlayEntries), p.Amount),
			Value: fmt.Sprintf("%d", p.ID),
		})
		if len(options) == 25 {
			break
		}
	}
	if len(options) == 0 {
		return respondEphemeral(s, i, "You have no open 4-leg parlay to insure. The card stays in your inventory.")
	}
	minValues := 1
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Select a parlay for Speakeasy.",
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						MenuType:    discordgo.StringSelectMenu,
						CustomID:    fmt.Sprintf("synparlay_%d_%s_%s", cards.SpeakeasyParlayCardID, user.DiscordID, user.GuildID),
						Placeholder: "Choose a parlay...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options:     options,
					},
				}},
			},
		},
	})
}

func ShowOpponentMenu(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, user models.User, entry entryChoice) error {
	type opp struct {
		DiscordID string
		Username  *string
	}
	var opps []opp
	err := db.Table("bet_entries").
		Select("DISTINCT users.discord_id, users.username").
		Joins("JOIN users ON users.id = bet_entries.user_id").
		Where("bet_entries.bet_id = ? AND bet_entries.option <> ? AND bet_entries.deleted_at IS NULL AND bet_entries.user_id <> ?",
			entry.BetID, entry.Option, user.ID).
		Limit(25).Scan(&opps).Error
	if err != nil {
		return err
	}
	if len(opps) == 0 {
		return respondEphemeral(s, i, "Nobody is on the other side of that entry. The card stays in your inventory.")
	}
	options := make([]discordgo.SelectMenuOption, 0, len(opps))
	for _, o := range opps {
		label := o.DiscordID
		if o.Username != nil && *o.Username != "" {
			label = *o.Username
		}
		if len(label) > 100 {
			label = label[:97] + "..."
		}
		options = append(options, discordgo.SelectMenuOption{Label: label, Value: o.DiscordID})
	}
	minValues := 1
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Select the bettor on the other side.",
			Flags:   discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						MenuType:    discordgo.StringSelectMenu,
						CustomID:    fmt.Sprintf("synopp_%d_%s_%s", entry.EntryID, user.DiscordID, user.GuildID),
						Placeholder: "Choose a mark...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options:     options,
					},
				}},
			},
		},
	})
}

func InventorySummary(db *gorm.DB, userDBID uint, guildID string) (string, error) {
	type row struct {
		Name  string
		Count int
	}
	var rows []row
	err := db.Table("user_inventories").
		Select("cards.name as name, COUNT(*) as count").
		Joins("JOIN cards ON cards.id = user_inventories.card_id").
		Where("user_inventories.user_id = ? AND user_inventories.guild_id = ? AND user_inventories.deleted_at IS NULL", userDBID, guildID).
		Group("cards.name").
		Order("cards.name").
		Scan(&rows).Error
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "Their pockets are empty.", nil
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s x%d\n", r.Name, r.Count)
	}
	text := b.String()
	if len(text) > 1800 {
		text = text[:1800] + "..."
	}
	return text, nil
}

func loadEntryChoice(db *gorm.DB, entryID uint, userDBID uint, guildID string) (entryChoice, error) {
	var row entryChoice
	err := db.Table("bet_entries").
		Select(`bet_entries.id as entry_id, bet_entries.option, bet_entries.amount, bet_entries.spread,
			bets.id as bet_id, bets.description, bets.option1, bets.option2, bets.active, bets.paid,
			bets.cfbd_id, bets.espn_id, bets.game_start_date as game_start, bets.opening_spread, bets.spread as bet_spread`).
		Joins("JOIN bets ON bets.id = bet_entries.bet_id AND bets.deleted_at IS NULL").
		Where("bet_entries.id = ? AND bet_entries.user_id = ? AND bet_entries.deleted_at IS NULL AND bets.guild_id = ?", entryID, userDBID, guildID).
		Scan(&row).Error
	if err != nil {
		return row, err
	}
	if row.EntryID == 0 {
		return row, fmt.Errorf("entry not found")
	}
	return row, nil
}

func toBetEntry(row entryChoice) models.BetEntry {
	return models.BetEntry{
		ID:     row.EntryID,
		BetID:  row.BetID,
		Option: row.Option,
		Amount: row.Amount,
		Spread: row.Spread,
	}
}

func sportsSpread(row entryChoice) bool {
	return row.BetSpread != nil && (row.CfbdID != nil || row.EspnID != nil)
}
