package cardSelection

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"perfectOddsBot/models"
	cardService "perfectOddsBot/services/cardService"
	"perfectOddsBot/services/cardService/cards"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func loadPlayer(db *gorm.DB, discordID, guildID string) (models.User, error) {
	var user models.User
	err := db.Where("discord_id = ? AND guild_id = ?", discordID, guildID).First(&user).Error
	return user, err
}

func samePlayer(s *discordgo.Session, i *discordgo.InteractionCreate, discordID string) bool {
	return i.Member != nil && i.Member.User != nil && i.Member.User.ID == discordID
}

// BeginSyndicatePlay starts the menu or the self-effect for a held Syndicate card.
func BeginSyndicatePlay(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, cardID uint, discordID, guildID string) error {
	user, err := loadPlayer(db, discordID, guildID)
	if err != nil {
		return err
	}
	switch cardID {
	case cards.CookedBooksCardID:
		err = db.Transaction(func(tx *gorm.DB) error {
			return cards.StartCookedBooks(tx, user)
		})
		if err != nil {
			return err
		}
		return respondEphemeral(s, i, "Cooked Books are open for 24 hours. Point thieves cannot find the real ledger.")
	case cards.BagmansCutCardID:
		err = db.Transaction(func(tx *gorm.DB) error {
			return cards.StartBagman(tx, user)
		})
		if err != nil {
			return err
		}
		return respondEphemeral(s, i, "Bagman's Cut is open for 12 hours. Fifteen percent of every draw and purchase comes to you.")
	case cards.WateredDownHoochCardID, cards.LookoutBoyCardID, cards.ProtectionRacketCardID, cards.ConcreteShoesCardID, cards.LeveragedBuyoutCardID, cards.TheDonsFavorCardID:
		cardService.ShowUserSelectMenu(s, i, cardID, cardName(cardID), "Choose a user. You cannot target yourself.", discordID, guildID, db)
		return nil
	case cards.BackroomWiretapCardID:
		return ShowWiretapChoice(s, i, user)
	case cards.SpeakeasyParlayCardID:
		return ShowParlayMenu(s, i, db, user)
	case cards.BrassKnucklesCardID, cards.LoadedDiceCardID, cards.PointShaverCardID, cards.TheFixIsInCardID:
		return ShowSyndicateEntryMenu(s, i, db, user, cardID)
	default:
		return respondEphemeral(s, i, "This card type is not yet supported for manual play.")
	}
}

func cardName(cardID uint) string {
	if card := cardService.GetCardByID(cardID); card != nil {
		return card.Name
	}
	return "Card"
}

func HandleSyndicateUserPlay(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, cardID int, discordID, guildID, targetID string) error {
	if targetID == discordID {
		return respondEphemeral(s, i, "You cannot target yourself. The card stays in your inventory.")
	}
	user, err := loadPlayer(db, discordID, guildID)
	if err != nil {
		return err
	}
	target, err := loadPlayer(db, targetID, guildID)
	if err != nil {
		return respondEphemeral(s, i, "That user is not in this server. The card stays in your inventory.")
	}
	switch uint(cardID) {
	case cards.WateredDownHoochCardID:
		err = db.Transaction(func(tx *gorm.DB) error {
			return cards.PlaceCurseOnTarget(tx, user, target, cards.WateredDownHoochCardID, "HCH", nil)
		})
		if err != nil {
			return err
		}
		return publicPlay(s, i, db, user, cards.WateredDownHoochCardID, fmt.Sprintf("<@%s> slipped Watered-Down Hooch to <@%s>. Their next draw or purchase costs 25%% more.", user.DiscordID, target.DiscordID))
	case cards.ConcreteShoesCardID:
		until := time.Now().Add(12 * time.Hour)
		err = db.Transaction(func(tx *gorm.DB) error {
			return cards.PlaceCurseOnTarget(tx, user, target, cards.ConcreteShoesCardID, "CSH", &until)
		})
		if err != nil {
			return err
		}
		return publicPlay(s, i, db, user, cards.ConcreteShoesCardID, fmt.Sprintf("<@%s> put <@%s> in Concrete Shoes for 12 hours. No store, no transfer portal.", user.DiscordID, target.DiscordID))
	case cards.ProtectionRacketCardID:
		err = db.Transaction(func(tx *gorm.DB) error {
			return cards.PlaceRacket(tx, user, target)
		})
		if err != nil {
			return err
		}
		return publicPlay(s, i, db, user, cards.ProtectionRacketCardID, fmt.Sprintf("<@%s> put a Protection Racket on <@%s>.", user.DiscordID, target.DiscordID))
	case cards.TheDonsFavorCardID:
		err = db.Transaction(func(tx *gorm.DB) error {
			return cards.PlaceDon(tx, user, target)
		})
		if err != nil {
			return err
		}
		return publicPlay(s, i, db, user, cards.TheDonsFavorCardID, fmt.Sprintf("<@%s> made <@%s> a silent partner for 24 hours.", user.DiscordID, target.DiscordID))
	case cards.LookoutBoyCardID:
		summary, err := InventorySummary(db, target.ID, guildID)
		if err != nil {
			return err
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			return cards.ConsumeHeldCopy(tx, user.ID, guildID, cards.LookoutBoyCardID)
		}); err != nil {
			return err
		}
		if err := respondEphemeral(s, i, "🥃 Lookout Boy\n"+summary); err != nil {
			return err
		}
		return cardService.NotifyCardPlayedWithMessage(s, db, user, cardOrStub(cards.LookoutBoyCardID), fmt.Sprintf("<@%s> sent a Lookout Boy after <@%s>.", user.DiscordID, target.DiscordID))
	case cards.LeveragedBuyoutCardID:
		var given, taken string
		var ok bool
		err = db.Transaction(func(tx *gorm.DB) error {
			var swapErr error
			given, taken, ok, swapErr = cards.LeveragedBuyoutSwap(tx, user, target)
			return swapErr
		})
		if err != nil {
			return err
		}
		if !ok {
			return respondEphemeral(s, i, "The buyout fizzles. You need an unattached Common, and they need an unattached Rare or Epic. The card stays in your inventory.")
		}
		return publicPlay(s, i, db, user, cards.LeveragedBuyoutCardID, fmt.Sprintf("<@%s> swapped **%s** for <@%s>'s **%s**.", user.DiscordID, given, target.DiscordID, taken))
	default:
		return respondEphemeral(s, i, "That card cannot target a user.")
	}
}

func publicPlay(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, user models.User, cardID uint, message string) error {
	if err := respondEphemeral(s, i, message); err != nil {
		return err
	}
	card := cardOrStub(cardID)
	return cardService.NotifyCardPlayedWithMessage(s, db, user, card, message)
}

func cardOrStub(cardID uint) *models.Card {
	if card := cardService.GetCardByID(cardID); card != nil {
		return card
	}
	return &models.Card{ID: cardID, Name: cardName(cardID)}
}

func HandleSynEntry(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	parts := strings.Split(i.MessageComponentData().CustomID, "_")
	if len(parts) != 5 {
		return fmt.Errorf("invalid syndicate entry id")
	}
	mode := parts[1]
	cardID64, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return err
	}
	discordID, guildID := parts[3], parts[4]
	if !samePlayer(s, i, discordID) {
		return respondEphemeral(s, i, "You can only play your own cards.")
	}
	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no entry selected")
	}
	entryID64, err := strconv.ParseUint(i.MessageComponentData().Values[0], 10, 64)
	if err != nil {
		return err
	}
	user, err := loadPlayer(db, discordID, guildID)
	if err != nil {
		return err
	}
	row, err := loadEntryChoice(db, uint(entryID64), user.ID, guildID)
	if err != nil {
		return respondEphemeral(s, i, "That entry is no longer available. The card stays in your inventory.")
	}
	cardID := uint(cardID64)
	entry := toBetEntry(row)
	switch mode {
	case "active":
		if !row.Active || row.Paid {
			return respondEphemeral(s, i, "That bet is no longer open. The card stays in your inventory.")
		}
		if cardID == cards.BrassKnucklesCardID {
			return ShowOpponentMenu(s, i, db, user, row)
		}
		if cardID == cards.LoadedDiceCardID {
			roll := cards.RollLoadedDice()
			var placed bool
			err = db.Transaction(func(tx *gorm.DB) error {
				var placeErr error
				placed, placeErr = cards.PlaceDice(tx, user, entry, roll)
				return placeErr
			})
			if err != nil {
				return err
			}
			if !placed {
				return respondEphemeral(s, i, "That entry already has Loaded Dice. The card stays in your inventory.")
			}
			return publicPlay(s, i, db, user, cardID, fmt.Sprintf("<@%s> rolled Loaded Dice on **%s**: **%.2fx**.", discordID, row.Description, roll))
		}
	case "shave":
		if !row.Active || !sportsSpread(row) || row.Spread == nil {
			return respondEphemeral(s, i, "That entry cannot be shaved. The card stays in your inventory.")
		}
		err = db.Transaction(func(tx *gorm.DB) error {
			return cards.ShaveEntry(tx, user, entry)
		})
		if err != nil {
			return err
		}
		return publicPlay(s, i, db, user, cards.PointShaverCardID, fmt.Sprintf("<@%s> shaved 1.5 points off **%s**.", discordID, row.Description))
	case "fix":
		if row.Paid || !sportsSpread(row) {
			return respondEphemeral(s, i, "The Fix only attaches to an unpaid CFB or CBB spread entry. The card stays in your inventory.")
		}
		var placed bool
		err = db.Transaction(func(tx *gorm.DB) error {
			var placeErr error
			placed, placeErr = cards.PlaceFix(tx, user, entry)
			return placeErr
		})
		if err != nil {
			return err
		}
		if !placed {
			return respondEphemeral(s, i, "You already have The Fix Is In on that bet. The card stays in your inventory.")
		}
		return publicPlay(s, i, db, user, cards.TheFixIsInCardID, fmt.Sprintf("<@%s> bought The Fix on **%s**.", discordID, row.Description))
	case "open":
		if !row.Active || row.BetSpread == nil {
			return respondEphemeral(s, i, "That entry is not an open spread. The card stays in your inventory.")
		}
		var placed bool
		err = db.Transaction(func(tx *gorm.DB) error {
			var placeErr error
			placed, placeErr = cards.OpenLineEntry(tx, user, entry, row.OpeningSpread)
			return placeErr
		})
		if err != nil {
			return err
		}
		if !placed {
			return respondEphemeral(s, i, "That bet has no opening line on file. The card stays in your inventory.")
		}
		return publicPlay(s, i, db, user, cards.BackroomWiretapCardID, fmt.Sprintf("<@%s> locked **%s** to the opening line.", discordID, row.Description))
	case "refund":
		if row.Active || row.GameStart == nil || !time.Now().Before(*row.GameStart) {
			return respondEphemeral(s, i, "That entry cannot be refunded. The card stays in your inventory.")
		}
		err = db.Transaction(func(tx *gorm.DB) error {
			locked := user
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, user.ID).Error; err != nil {
				return err
			}
			return cards.RefundEntry(tx, locked, entry)
		})
		if err != nil {
			return err
		}
		return publicPlay(s, i, db, user, cards.BackroomWiretapCardID, fmt.Sprintf("<@%s> refunded %d points on **%s** before kickoff.", discordID, entry.Amount, row.Description))
	}
	return respondEphemeral(s, i, "That play is not available.")
}

func HandleSynWire(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	parts := strings.Split(i.MessageComponentData().CustomID, "_")
	if len(parts) != 4 {
		return fmt.Errorf("invalid wiretap id")
	}
	discordID, guildID := parts[2], parts[3]
	if !samePlayer(s, i, discordID) {
		return respondEphemeral(s, i, "You can only play your own cards.")
	}
	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no wiretap mode")
	}
	user, err := loadPlayer(db, discordID, guildID)
	if err != nil {
		return err
	}
	return ShowWiretapEntries(s, i, db, user, i.MessageComponentData().Values[0])
}

func HandleSynOpp(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	parts := strings.Split(i.MessageComponentData().CustomID, "_")
	if len(parts) != 4 {
		return fmt.Errorf("invalid knuckles id")
	}
	entryID64, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return err
	}
	discordID, guildID := parts[2], parts[3]
	if !samePlayer(s, i, discordID) {
		return respondEphemeral(s, i, "You can only play your own cards.")
	}
	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no opponent")
	}
	targetID := i.MessageComponentData().Values[0]
	user, err := loadPlayer(db, discordID, guildID)
	if err != nil {
		return err
	}
	row, err := loadEntryChoice(db, uint(entryID64), user.ID, guildID)
	if err != nil || !row.Active {
		return respondEphemeral(s, i, "That entry is no longer open. The card stays in your inventory.")
	}
	var placed bool
	err = db.Transaction(func(tx *gorm.DB) error {
		var placeErr error
		placed, placeErr = cards.PlaceKnuckles(tx, user, toBetEntry(row), targetID)
		return placeErr
	})
	if err != nil {
		return err
	}
	if !placed {
		return respondEphemeral(s, i, "Brass Knuckles cannot attach there. The card stays in your inventory.")
	}
	return publicPlay(s, i, db, user, cards.BrassKnucklesCardID, fmt.Sprintf("<@%s> put Brass Knuckles on <@%s> for **%s**.", discordID, targetID, row.Description))
}

func HandleSynParlay(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) error {
	parts := strings.Split(i.MessageComponentData().CustomID, "_")
	if len(parts) != 4 {
		return fmt.Errorf("invalid parlay id")
	}
	discordID, guildID := parts[2], parts[3]
	if !samePlayer(s, i, discordID) {
		return respondEphemeral(s, i, "You can only play your own cards.")
	}
	if len(i.MessageComponentData().Values) == 0 {
		return fmt.Errorf("no parlay")
	}
	parlayID64, err := strconv.ParseUint(i.MessageComponentData().Values[0], 10, 64)
	if err != nil {
		return err
	}
	user, err := loadPlayer(db, discordID, guildID)
	if err != nil {
		return err
	}
	var parlay models.Parlay
	if err := db.Where("id = ? AND user_id = ? AND guild_id = ?", uint(parlayID64), user.ID, guildID).First(&parlay).Error; err != nil {
		return respondEphemeral(s, i, "That parlay is gone. The card stays in your inventory.")
	}
	var placed bool
	err = db.Transaction(func(tx *gorm.DB) error {
		var placeErr error
		placed, placeErr = cards.PlaceSpeakeasy(tx, user, parlay)
		return placeErr
	})
	if err != nil {
		return err
	}
	if !placed {
		return respondEphemeral(s, i, "That parlay cannot be insured. The card stays in your inventory.")
	}
	return publicPlay(s, i, db, user, cards.SpeakeasyParlayCardID, fmt.Sprintf("<@%s> insured a parlay with Speakeasy.", discordID))
}
