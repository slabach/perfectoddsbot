package cardService

import (
	"errors"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newMockDB() (*gorm.DB, sqlmock.Sqlmock, error) {
	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, nil, err
	}

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})

	return gormDB, mock, err
}

func TestApplyDoubleDownIfAvailable(t *testing.T) {
	t.Run("User has card", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		originalPayout := 100.0

		// Expect check for card
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.DoubleDownCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		// Mock consumer
		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			if u.ID != user.ID {
				t.Errorf("Expected user ID %d, got %d", user.ID, u.ID)
			}
			if cardID != cards.DoubleDownCardID {
				t.Errorf("Expected card ID %d, got %d", cards.DoubleDownCardID, cardID)
			}
			consumed = true
			return nil
		}

		payout, applied, err := ApplyDoubleDownIfAvailable(db, consumer, user, originalPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied")
		}
		if !consumed {
			t.Error("Expected consumer to be called")
		}
		if payout != originalPayout*2 {
			t.Errorf("Expected payout %.2f, got %.2f", originalPayout*2, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User does not have card", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		originalPayout := 100.0

		// Expect check for card - return 0
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.DoubleDownCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			t.Error("Consumer should not be called")
			return nil
		}

		payout, applied, err := ApplyDoubleDownIfAvailable(db, consumer, user, originalPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected card NOT to be applied")
		}
		if payout != originalPayout {
			t.Errorf("Expected payout %.2f, got %.2f", originalPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("DB Error", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		originalPayout := 100.0

		// Expect check for card - return error
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.DoubleDownCardID).
			WillReturnError(errors.New("db error"))

		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			t.Error("Consumer should not be called")
			return nil
		}

		payout, applied, err := ApplyDoubleDownIfAvailable(db, consumer, user, originalPayout)

		if err == nil {
			t.Error("Expected error")
		}
		if applied {
			t.Error("Expected card NOT to be applied")
		}
		if payout != originalPayout {
			t.Errorf("Expected payout %.2f, got %.2f", originalPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}

func TestApplyEmotionalHedgeIfApplicable(t *testing.T) {
	t.Run("Applicable and Refunded (Team Lost Straight Up)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		bet := models.Bet{Option1: "Team A", Option2: "Team B", GuildID: "guild1"}
		subscribedTeam := "Team A"

		// 1. Check user has card
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.EmotionalHedgeCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		// 2. Check guild subscription
		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(user.GuildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "guild_id", "subscribed_team"}).
				AddRow(1, "guild1", subscribedTeam))

		// User picks Team A (Option 1)
		userPick := 1
		betAmount := 100.0
		// Team A lost: Option 1 (Team A) score < Option 2 (Team B) score -> scoreDiff < 0
		scoreDiff := -10

		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			consumed = true
			return nil
		}

		refund, applied, err := ApplyEmotionalHedgeIfApplicable(db, consumer, user, bet, userPick, betAmount, scoreDiff)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied")
		}
		if !consumed {
			t.Error("Expected consumer to be called")
		}
		if refund != betAmount*0.5 {
			t.Errorf("Expected refund %.2f, got %.2f", betAmount*0.5, refund)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Applicable but No Refund (Team Won Straight Up)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		bet := models.Bet{Option1: "Team A", Option2: "Team B", GuildID: "guild1"}
		subscribedTeam := "Team A"

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(user.GuildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "guild_id", "subscribed_team"}).
				AddRow(1, "guild1", subscribedTeam))

		// User picks Team A
		userPick := 1
		betAmount := 100.0
		// Team A won: scoreDiff > 0
		scoreDiff := 10

		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			consumed = true
			return nil
		}

		refund, applied, err := ApplyEmotionalHedgeIfApplicable(db, consumer, user, bet, userPick, betAmount, scoreDiff)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied (consumed)")
		}
		if !consumed {
			t.Error("Expected consumer to be called")
		}
		if refund != 0 {
			t.Errorf("Expected refund 0, got %.2f", refund)
		}
	})

	t.Run("Not Applicable (User bet on other team)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		bet := models.Bet{Option1: "Team A", Option2: "Team B", GuildID: "guild1"}
		subscribedTeam := "Team A"

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(user.GuildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "guild_id", "subscribed_team"}).
				AddRow(1, "guild1", subscribedTeam))

		// User picks Team B (Option 2)
		userPick := 2
		betAmount := 100.0
		scoreDiff := -10

		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			t.Error("Consumer should not be called")
			return nil
		}

		refund, applied, err := ApplyEmotionalHedgeIfApplicable(db, consumer, user, bet, userPick, betAmount, scoreDiff)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected card NOT to be applied")
		}
		if refund != 0 {
			t.Errorf("Expected refund 0, got %.2f", refund)
		}
	})

	t.Run("Not Applicable (User does not have card)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		bet := models.Bet{Option1: "Team A", Option2: "Team B", GuildID: "guild1"}

		// Expect check for card - return 0
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.EmotionalHedgeCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			t.Error("Consumer should not be called")
			return nil
		}

		_, applied, _ := ApplyEmotionalHedgeIfApplicable(db, consumer, user, bet, 1, 100.0, -10)
		if applied {
			t.Error("Expected card NOT to be applied")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}

func TestApplyBetInsuranceIfApplicable(t *testing.T) {
	t.Run("User has insurance card and loses bet", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		betAmount := 100.0

		// Expect check for card
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.BetInsuranceCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		// Mock consumer
		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			if u.ID != user.ID {
				t.Errorf("Expected user ID %d, got %d", user.ID, u.ID)
			}
			if cardID != cards.BetInsuranceCardID {
				t.Errorf("Expected card ID %d, got %d", cards.BetInsuranceCardID, cardID)
			}
			consumed = true
			return nil
		}

		refund, applied, err := ApplyBetInsuranceIfApplicable(db, consumer, user, betAmount, false)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied")
		}
		if !consumed {
			t.Error("Expected consumer to be called")
		}
		if refund != betAmount*0.25 {
			t.Errorf("Expected refund %.2f, got %.2f", betAmount*0.25, refund)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User has insurance card and wins bet", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		betAmount := 100.0

		// Expect check for card
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.BetInsuranceCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			consumed = true
			return nil
		}

		refund, applied, err := ApplyBetInsuranceIfApplicable(db, consumer, user, betAmount, true)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied (fizzle)")
		}
		if !consumed {
			t.Error("Expected consumer to be called")
		}
		if refund != 0 {
			t.Errorf("Expected refund 0, got %.2f", refund)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User does not have insurance card", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		betAmount := 100.0

		// Expect check for card - return 0
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.BetInsuranceCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			t.Error("Consumer should not be called")
			return nil
		}

		refund, applied, err := ApplyBetInsuranceIfApplicable(db, consumer, user, betAmount, false)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected card NOT to be applied")
		}
		if refund != 0 {
			t.Errorf("Expected refund 0, got %.2f", refund)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("DB Error", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		betAmount := 100.0

		// Expect check for card - return error
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.BetInsuranceCardID).
			WillReturnError(errors.New("db error"))

		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			t.Error("Consumer should not be called")
			return nil
		}

		_, applied, err := ApplyBetInsuranceIfApplicable(db, consumer, user, betAmount, false)

		if err == nil {
			t.Error("Expected error")
		}
		if applied {
			t.Error("Expected card NOT to be applied")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}

func TestApplyUnoReverseIfApplicable(t *testing.T) {
	t.Run("Card exists (Win -> Loss)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		betID := uint(123)

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.UnoReverseCardID, betID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "target_bet_id"}).
				AddRow(1, user.ID, user.GuildID, cards.UnoReverseCardID, betID))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		applied, newIsWin, err := ApplyUnoReverseIfApplicable(db, user, betID, true)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied")
		}
		if newIsWin {
			t.Error("Expected newIsWin to be false (flipped from true)")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Card exists (Loss -> Win)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		betID := uint(123)

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.UnoReverseCardID, betID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "target_bet_id"}).
				AddRow(1, user.ID, user.GuildID, cards.UnoReverseCardID, betID))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		applied, newIsWin, err := ApplyUnoReverseIfApplicable(db, user, betID, false)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied")
		}
		if !newIsWin {
			t.Error("Expected newIsWin to be true (flipped from false)")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Card does not exist", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		betID := uint(123)

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.UnoReverseCardID, betID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "target_bet_id"})) // Empty result

		applied, newIsWin, err := ApplyUnoReverseIfApplicable(db, user, betID, true)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected card NOT to be applied")
		}
		if !newIsWin {
			t.Error("Expected newIsWin to remain true")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}

func TestApplyAntiAntiBetIfApplicable(t *testing.T) {
	t.Run("Card exists, target user wins bet (card holder loses)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		bettorUser := models.User{ID: 1, DiscordID: "target123", GuildID: "guild1"}
		targetDiscordID := "target123"

		cardHolderID := uint(2)
		cardHolderDiscordID := "holder123"
		betAmount := 100.0

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(bettorUser.GuildID, cards.AntiAntiBetCardID, targetDiscordID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "target_user_id", "bet_amount"}).
				AddRow(1, cardHolderID, "guild1", cards.AntiAntiBetCardID, targetDiscordID, betAmount))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(cardHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(cardHolderID, cardHolderDiscordID, "guild1", 500.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		payout, winners, losers, applied, err := ApplyAntiAntiBetIfApplicable(db, bettorUser, true)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied")
		}
		if payout != 0 {
			t.Errorf("Expected payout 0 (holder loses), got %.2f", payout)
		}
		if len(winners) != 0 {
			t.Errorf("Expected no winners when target wins, got %d", len(winners))
		}
		if len(losers) != 1 {
			t.Errorf("Expected 1 loser, got %d", len(losers))
		}
		if len(losers) > 0 && losers[0].DiscordID != cardHolderDiscordID {
			t.Errorf("Expected loser DiscordID '%s', got '%s'", cardHolderDiscordID, losers[0].DiscordID)
		}
		if len(losers) > 0 && losers[0].Payout != betAmount {
			t.Errorf("Expected loser payout %.2f (bet amount), got %.2f", betAmount, losers[0].Payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Card exists, target user loses bet (card holder wins)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		bettorUser := models.User{ID: 1, DiscordID: "target123", GuildID: "guild1"}
		targetDiscordID := "target123"
		cardHolderID := uint(2)
		betAmount := 100.0

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(bettorUser.GuildID, cards.AntiAntiBetCardID, targetDiscordID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "target_user_id", "bet_amount"}).
				AddRow(1, cardHolderID, "guild1", cards.AntiAntiBetCardID, targetDiscordID, betAmount))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(cardHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(cardHolderID, "holder123", "guild1", 500.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), cardHolderID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		payout, winners, losers, applied, err := ApplyAntiAntiBetIfApplicable(db, bettorUser, false)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied")
		}
		expectedPayout := betAmount * 2.0 // Even odds (+100)
		if payout != expectedPayout {
			t.Errorf("Expected payout %.2f, got %.2f", expectedPayout, payout)
		}
		if len(winners) != 1 {
			t.Errorf("Expected 1 winner, got %d", len(winners))
		}
		if len(losers) != 0 {
			t.Errorf("Expected no losers, got %d", len(losers))
		}
		if len(winners) > 0 && winners[0].DiscordID != "holder123" {
			t.Errorf("Expected winner DiscordID 'holder123', got '%s'", winners[0].DiscordID)
		}
		if len(winners) > 0 && winners[0].Payout != expectedPayout {
			t.Errorf("Expected winner payout %.2f, got %.2f", expectedPayout, winners[0].Payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Multiple cards exist for same target", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		bettorUser := models.User{ID: 1, DiscordID: "target123", GuildID: "guild1"}
		targetDiscordID := "target123"
		cardHolder1ID := uint(2)
		cardHolder2ID := uint(3)
		betAmount1 := 100.0
		betAmount2 := 50.0

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(bettorUser.GuildID, cards.AntiAntiBetCardID, targetDiscordID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "target_user_id", "bet_amount"}).
				AddRow(1, cardHolder1ID, "guild1", cards.AntiAntiBetCardID, targetDiscordID, betAmount1).
				AddRow(2, cardHolder2ID, "guild1", cards.AntiAntiBetCardID, targetDiscordID, betAmount2))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(cardHolder1ID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(cardHolder1ID, "holder123", "guild1", 500.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), cardHolder1ID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(cardHolder2ID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(cardHolder2ID, "holder456", "guild1", 300.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), cardHolder2ID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 2).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		payout, winners, losers, applied, err := ApplyAntiAntiBetIfApplicable(db, bettorUser, false)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied")
		}
		expectedPayout := (betAmount1 * 2.0) + (betAmount2 * 2.0)
		if payout != expectedPayout {
			t.Errorf("Expected payout %.2f, got %.2f", expectedPayout, payout)
		}
		if len(winners) != 2 {
			t.Errorf("Expected 2 winners, got %d", len(winners))
		}
		if len(losers) != 0 {
			t.Errorf("Expected no losers, got %d", len(losers))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Card does not exist", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		bettorUser := models.User{ID: 1, DiscordID: "target123", GuildID: "guild1"}
		targetDiscordID := "target123"

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(bettorUser.GuildID, cards.AntiAntiBetCardID, targetDiscordID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "target_user_id", "bet_amount"}))

		payout, winners, losers, applied, err := ApplyAntiAntiBetIfApplicable(db, bettorUser, true)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected card NOT to be applied")
		}
		if payout != 0 {
			t.Errorf("Expected payout 0, got %.2f", payout)
		}
		if len(winners) != 0 {
			t.Errorf("Expected no winners, got %d", len(winners))
		}
		if len(losers) != 0 {
			t.Errorf("Expected no losers, got %d", len(losers))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("DB Error", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		bettorUser := models.User{ID: 1, DiscordID: "target123", GuildID: "guild1"}
		targetDiscordID := "target123"

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(bettorUser.GuildID, cards.AntiAntiBetCardID, targetDiscordID).
			WillReturnError(errors.New("db error"))

		_, _, _, applied, err := ApplyAntiAntiBetIfApplicable(db, bettorUser, true)

		if err == nil {
			t.Error("Expected error")
		}
		if applied {
			t.Error("Expected card NOT to be applied")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}

func TestApplyVampireIfApplicable(t *testing.T) {
	t.Run("Single vampire card exists, multiple winners (vampire holder gets 1% of total)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		totalWinningPayouts := 1000.0
		expectedVampirePayout := 10.0 // 1% of 1000

		vampireHolderID := uint(2)
		vampireHolderDiscordID := "vampire123"

		// Mock query for active vampire cards (created_at >= 24 hours ago)
		createdAt := time.Now().Add(-12 * time.Hour) // Less than 24 hours ago, so it's active
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.VampireCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, vampireHolderID, guildID, cards.VampireCardID, nil, nil, 0.0))

		// Mock query for vampire holder user
		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(vampireHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(vampireHolderID, vampireHolderDiscordID, guildID, 500.0))

		// Mock update for vampire holder points
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), vampireHolderID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs["winner123"] = 500.0 // Other winner with $500 payout, not the vampire holder
		totalVampirePayout, winners, applied, err := ApplyVampireIfApplicable(db, guildID, totalWinningPayouts, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected vampire to be applied")
		}
		if totalVampirePayout != expectedVampirePayout {
			t.Errorf("Expected total vampire payout %.2f, got %.2f", expectedVampirePayout, totalVampirePayout)
		}
		if len(winners) != 1 {
			t.Errorf("Expected 1 winner, got %d", len(winners))
		}
		if len(winners) > 0 && winners[0].DiscordID != vampireHolderDiscordID {
			t.Errorf("Expected winner DiscordID '%s', got '%s'", vampireHolderDiscordID, winners[0].DiscordID)
		}
		if len(winners) > 0 && winners[0].Payout != expectedVampirePayout {
			t.Errorf("Expected winner payout %.2f, got %.2f", expectedVampirePayout, winners[0].Payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Multiple vampire cards exist for different holders (each gets 1% independently)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		totalWinningPayouts := 350.0
		expectedVampirePayout := 3.5 // 1% of 350

		vampireHolder1ID := uint(2)
		vampireHolder1DiscordID := "vampire123"
		vampireHolder2ID := uint(3)
		vampireHolder2DiscordID := "vampire456"

		// Mock query for active vampire cards
		createdAt1 := time.Now().Add(-12 * time.Hour) // Less than 24 hours ago, so it's active
		createdAt2 := time.Now().Add(-12 * time.Hour) // Less than 24 hours ago, so it's active
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.VampireCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt1, createdAt1, nil, vampireHolder1ID, guildID, cards.VampireCardID, nil, nil, 0.0).
				AddRow(2, createdAt2, createdAt2, nil, vampireHolder2ID, guildID, cards.VampireCardID, nil, nil, 0.0))

		// Mock query for first vampire holder
		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(vampireHolder1ID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(vampireHolder1ID, vampireHolder1DiscordID, guildID, 500.0))

		// Mock update for first vampire holder
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), vampireHolder1ID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		// Mock query for second vampire holder
		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(vampireHolder2ID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(vampireHolder2ID, vampireHolder2DiscordID, guildID, 600.0))

		// Mock update for second vampire holder
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), vampireHolder2ID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs["winner123"] = 500.0 // Other winner with $500 payout, not the vampire holder
		totalVampirePayout, winners, applied, err := ApplyVampireIfApplicable(db, guildID, totalWinningPayouts, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected vampire to be applied")
		}
		expectedTotalPayout := expectedVampirePayout * 2 // Two vampires
		if totalVampirePayout != expectedTotalPayout {
			t.Errorf("Expected total vampire payout %.2f, got %.2f", expectedTotalPayout, totalVampirePayout)
		}
		if len(winners) != 2 {
			t.Errorf("Expected 2 winners, got %d", len(winners))
		}
		if len(winners) > 0 && winners[0].Payout != expectedVampirePayout {
			t.Errorf("Expected first winner payout %.2f, got %.2f", expectedVampirePayout, winners[0].Payout)
		}
		if len(winners) > 1 && winners[1].Payout != expectedVampirePayout {
			t.Errorf("Expected second winner payout %.2f, got %.2f", expectedVampirePayout, winners[1].Payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("No vampire cards exist (no effect)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		totalWinningPayouts := 1000.0

		// Mock query returning no rows
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.VampireCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs["winner123"] = 500.0 // Other winner with $500 payout, not the vampire holder
		totalVampirePayout, winners, applied, err := ApplyVampireIfApplicable(db, guildID, totalWinningPayouts, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected vampire NOT to be applied")
		}
		if totalVampirePayout != 0 {
			t.Errorf("Expected payout 0, got %.2f", totalVampirePayout)
		}
		if len(winners) != 0 {
			t.Errorf("Expected no winners, got %d", len(winners))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("No winning payouts (vampires get nothing)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		totalWinningPayouts := 0.0
		expectedVampirePayout := 0.0 // 1% of 0

		vampireHolderID := uint(2)
		vampireHolderDiscordID := "vampire123"

		// Mock query for active vampire cards
		createdAt := time.Now().Add(-12 * time.Hour) // Less than 24 hours ago, so it's active
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.VampireCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, vampireHolderID, guildID, cards.VampireCardID, nil, nil, 0.0))

		// Mock query for vampire holder user
		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(vampireHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(vampireHolderID, vampireHolderDiscordID, guildID, 500.0))

		// Mock update for vampire holder points (even though it's 0, it still executes)
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), vampireHolderID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		// No winners when totalWinningPayouts = 0
		totalVampirePayout, winners, applied, err := ApplyVampireIfApplicable(db, guildID, totalWinningPayouts, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected vampire to be applied (even if payout is 0)")
		}
		if totalVampirePayout != expectedVampirePayout {
			t.Errorf("Expected total vampire payout %.2f, got %.2f", expectedVampirePayout, totalVampirePayout)
		}
		if len(winners) != 1 {
			t.Errorf("Expected 1 winner (even if payout is 0), got %d", len(winners))
		}
		if len(winners) > 0 && winners[0].Payout != expectedVampirePayout {
			t.Errorf("Expected winner payout %.2f, got %.2f", expectedVampirePayout, winners[0].Payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Vampire card holder is also a winner (should be excluded)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		totalWinningPayouts := 1000.0

		vampireHolderID := uint(2)
		vampireHolderDiscordID := "vampire123"

		// Mock query for active vampire cards
		createdAt := time.Now().Add(-12 * time.Hour) // Less than 24 hours ago, so it's active
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.VampireCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, vampireHolderID, guildID, cards.VampireCardID, nil, nil, 0.0))

		// Mock query for vampire holder user
		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(vampireHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(vampireHolderID, vampireHolderDiscordID, guildID, 500.0))

		// Note: No update should be executed because the vampire holder is a winner

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[vampireHolderDiscordID] = 1000.0 // Vampire holder won $1000
		totalVampirePayout, winners, applied, err := ApplyVampireIfApplicable(db, guildID, totalWinningPayouts, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected vampire to be applied (even if holder is excluded)")
		}
		if totalVampirePayout != 0 {
			t.Errorf("Expected total vampire payout 0 (holder excluded), got %.2f", totalVampirePayout)
		}
		if len(winners) != 0 {
			t.Errorf("Expected no winners (holder excluded), got %d", len(winners))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("DB Error", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		totalWinningPayouts := 1000.0

		// Mock query error
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.VampireCardID, sqlmock.AnyArg()).
			WillReturnError(errors.New("db error"))

		winnerDiscordIDs := make(map[string]float64)
		_, _, applied, err := ApplyVampireIfApplicable(db, guildID, totalWinningPayouts, winnerDiscordIDs)

		if err == nil {
			t.Error("Expected error")
		}
		if applied {
			t.Error("Expected vampire NOT to be applied")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}

func TestApplyGamblerIfAvailable(t *testing.T) {
	t.Run("User has card and wins (card consumed, payout may be doubled)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		originalPayout := 100.0

		// Expect check for card
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.GamblerCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		// Mock consumer
		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			if u.ID != user.ID {
				t.Errorf("Expected user ID %d, got %d", user.ID, u.ID)
			}
			if cardID != cards.GamblerCardID {
				t.Errorf("Expected card ID 72, got %d", cardID)
			}
			consumed = true
			return nil
		}

		payout, applied, err := ApplyGamblerIfAvailable(db, consumer, user, originalPayout, true)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied")
		}
		if !consumed {
			t.Error("Expected consumer to be called")
		}
		// Payout may be original or doubled (50/50), but should always be consumed
		if payout != originalPayout && payout != originalPayout*2 {
			t.Errorf("Expected payout to be %.2f or %.2f, got %.2f", originalPayout, originalPayout*2, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User has card and loses (card consumed, loss may be doubled)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		originalLoss := -100.0 // Negative for loss

		// Expect check for card
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.GamblerCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		// Mock consumer
		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			consumed = true
			return nil
		}

		loss, applied, err := ApplyGamblerIfAvailable(db, consumer, user, originalLoss, false)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected card to be applied")
		}
		if !consumed {
			t.Error("Expected consumer to be called")
		}
		// Loss may be original or doubled (50/50), but should always be consumed
		if loss != originalLoss && loss != originalLoss*2 {
			t.Errorf("Expected loss to be %.2f or %.2f, got %.2f", originalLoss, originalLoss*2, loss)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User does not have card", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		originalPayout := 100.0

		// Expect check for card - return 0
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.GamblerCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			t.Error("Consumer should not be called")
			return nil
		}

		payout, applied, err := ApplyGamblerIfAvailable(db, consumer, user, originalPayout, true)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected card NOT to be applied")
		}
		if payout != originalPayout {
			t.Errorf("Expected payout %.2f, got %.2f", originalPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("DB Error", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		originalPayout := 100.0

		// Expect check for card - return error
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.GamblerCardID).
			WillReturnError(errors.New("db error"))

		consumer := func(db *gorm.DB, u models.User, cardID int) error {
			t.Error("Consumer should not be called")
			return nil
		}

		payout, applied, err := ApplyGamblerIfAvailable(db, consumer, user, originalPayout, true)

		if err == nil {
			t.Error("Expected error")
		}
		if applied {
			t.Error("Expected card NOT to be applied")
		}
		if payout != originalPayout {
			t.Errorf("Expected payout %.2f, got %.2f", originalPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}
