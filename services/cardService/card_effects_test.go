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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.DoubleDownCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.DoubleDownCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.DoubleDownCardID).
			WillReturnError(errors.New("db error"))

		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.EmotionalHedgeCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(user.GuildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "guild_id", "subscribed_team"}).
				AddRow(1, "guild1", subscribedTeam))

		userPick := 1
		betAmount := 100.0
		scoreDiff := -10

		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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

		userPick := 1
		betAmount := 100.0
		scoreDiff := 10

		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
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

		userPick := 2
		betAmount := 100.0
		scoreDiff := -10

		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.EmotionalHedgeCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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

	t.Run("Applicable but No Refund (scoreDiff is 0 - unknown result)", func(t *testing.T) {
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

		userPick := 1
		betAmount := 100.0
		scoreDiff := 0

		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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
			t.Errorf("Expected refund 0 when scoreDiff is 0 (unknown result), got %.2f", refund)
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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.BetInsuranceCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.BetInsuranceCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.BetInsuranceCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.BetInsuranceCardID).
			WillReturnError(errors.New("db error"))

		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "target_bet_id"}))

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
		expectedPayout := betAmount * 2.0
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
	t.Run("Single vampire card exists, multiple winners (vampire holder gets 5% of total)", func(t *testing.T) {
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
		expectedVampirePayout := 50.0

		vampireHolderID := uint(2)
		vampireHolderDiscordID := "vampire123"

		createdAt := time.Now().Add(-12 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.VampireCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, vampireHolderID, guildID, cards.VampireCardID, nil, nil, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(vampireHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(vampireHolderID, vampireHolderDiscordID, guildID, 500.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), vampireHolderID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs["winner123"] = 500.0
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

	t.Run("Multiple vampire cards exist for different holders (each gets 5% independently)", func(t *testing.T) {
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
		expectedVampirePayout := 17.5

		vampireHolder1ID := uint(2)
		vampireHolder1DiscordID := "vampire123"
		vampireHolder2ID := uint(3)
		vampireHolder2DiscordID := "vampire456"

		createdAt1 := time.Now().Add(-12 * time.Hour)
		createdAt2 := time.Now().Add(-12 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.VampireCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt1, createdAt1, nil, vampireHolder1ID, guildID, cards.VampireCardID, nil, nil, 0.0).
				AddRow(2, createdAt2, createdAt2, nil, vampireHolder2ID, guildID, cards.VampireCardID, nil, nil, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(vampireHolder1ID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(vampireHolder1ID, vampireHolder1DiscordID, guildID, 500.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), vampireHolder1ID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(vampireHolder2ID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(vampireHolder2ID, vampireHolder2DiscordID, guildID, 600.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), vampireHolder2ID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs["winner123"] = 500.0
		totalVampirePayout, winners, applied, err := ApplyVampireIfApplicable(db, guildID, totalWinningPayouts, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected vampire to be applied")
		}
		expectedTotalPayout := expectedVampirePayout * 2
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

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.VampireCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs["winner123"] = 500.0
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
		expectedVampirePayout := 0.0

		vampireHolderID := uint(2)
		vampireHolderDiscordID := "vampire123"

		createdAt := time.Now().Add(-12 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.VampireCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, vampireHolderID, guildID, cards.VampireCardID, nil, nil, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(vampireHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(vampireHolderID, vampireHolderDiscordID, guildID, 500.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), vampireHolderID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
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

		createdAt := time.Now().Add(-12 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.VampireCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, vampireHolderID, guildID, cards.VampireCardID, nil, nil, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(vampireHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(vampireHolderID, vampireHolderDiscordID, guildID, 500.0))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[vampireHolderDiscordID] = 1000.0
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

func TestApplyTheLoversIfApplicable(t *testing.T) {
	t.Run("Single lovers card exists, target user wins (card holder gets 25%)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		targetUserDiscordID := "target123"
		targetWinnings := 1000.0
		expectedLoversPayout := 250.0

		loversHolderID := uint(2)
		loversHolderDiscordID := "lovers123"

		createdAt := time.Now().Add(-12 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheLoversCardID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, loversHolderID, guildID, cards.TheLoversCardID, nil, &targetUserDiscordID, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(loversHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(loversHolderID, loversHolderDiscordID, guildID, 500.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), loversHolderID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[targetUserDiscordID] = targetWinnings
		totalLoversPayout, winners, applied, err := ApplyTheLoversIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected The Lovers to be applied")
		}
		if totalLoversPayout != expectedLoversPayout {
			t.Errorf("Expected total lovers payout %.2f, got %.2f", expectedLoversPayout, totalLoversPayout)
		}
		if len(winners) != 1 {
			t.Errorf("Expected 1 winner, got %d", len(winners))
		}
		if len(winners) > 0 && winners[0].DiscordID != loversHolderDiscordID {
			t.Errorf("Expected winner DiscordID '%s', got '%s'", loversHolderDiscordID, winners[0].DiscordID)
		}
		if len(winners) > 0 && winners[0].Payout != expectedLoversPayout {
			t.Errorf("Expected winner payout %.2f, got %.2f", expectedLoversPayout, winners[0].Payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Multiple lovers cards exist for different targets (each gets 25% of their target's winnings)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		target1DiscordID := "target123"
		target2DiscordID := "target456"
		target1Winnings := 1000.0
		target2Winnings := 800.0
		expectedLovers1Payout := 250.0
		expectedLovers2Payout := 200.0

		loversHolder1ID := uint(2)
		loversHolder1DiscordID := "lovers123"
		loversHolder2ID := uint(3)
		loversHolder2DiscordID := "lovers456"

		createdAt1 := time.Now().Add(-12 * time.Hour)
		createdAt2 := time.Now().Add(-12 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheLoversCardID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt1, createdAt1, nil, loversHolder1ID, guildID, cards.TheLoversCardID, nil, &target1DiscordID, 0.0).
				AddRow(2, createdAt2, createdAt2, nil, loversHolder2ID, guildID, cards.TheLoversCardID, nil, &target2DiscordID, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(loversHolder1ID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(loversHolder1ID, loversHolder1DiscordID, guildID, 500.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), loversHolder1ID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(loversHolder2ID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(loversHolder2ID, loversHolder2DiscordID, guildID, 600.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), loversHolder2ID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[target1DiscordID] = target1Winnings
		winnerDiscordIDs[target2DiscordID] = target2Winnings
		totalLoversPayout, winners, applied, err := ApplyTheLoversIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected The Lovers to be applied")
		}
		expectedTotalPayout := expectedLovers1Payout + expectedLovers2Payout
		if totalLoversPayout != expectedTotalPayout {
			t.Errorf("Expected total lovers payout %.2f, got %.2f", expectedTotalPayout, totalLoversPayout)
		}
		if len(winners) != 2 {
			t.Errorf("Expected 2 winners, got %d", len(winners))
		}
		if len(winners) > 0 && winners[0].Payout != expectedLovers1Payout && winners[0].Payout != expectedLovers2Payout {
			t.Errorf("Expected first winner payout %.2f or %.2f, got %.2f", expectedLovers1Payout, expectedLovers2Payout, winners[0].Payout)
		}
		if len(winners) > 1 && winners[1].Payout != expectedLovers1Payout && winners[1].Payout != expectedLovers2Payout {
			t.Errorf("Expected second winner payout %.2f or %.2f, got %.2f", expectedLovers1Payout, expectedLovers2Payout, winners[1].Payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("No lovers cards exist (no effect)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheLoversCardID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs["winner123"] = 500.0
		totalLoversPayout, winners, applied, err := ApplyTheLoversIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected The Lovers NOT to be applied")
		}
		if totalLoversPayout != 0 {
			t.Errorf("Expected payout 0, got %.2f", totalLoversPayout)
		}
		if len(winners) != 0 {
			t.Errorf("Expected no winners, got %d", len(winners))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Target user didn't win (no payout)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		targetUserDiscordID := "target123"
		otherWinnerDiscordID := "winner456"

		loversHolderID := uint(2)

		createdAt := time.Now().Add(-12 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheLoversCardID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, loversHolderID, guildID, cards.TheLoversCardID, nil, &targetUserDiscordID, 0.0))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[otherWinnerDiscordID] = 1000.0
		totalLoversPayout, winners, applied, err := ApplyTheLoversIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected The Lovers to be applied (even if target didn't win)")
		}
		if totalLoversPayout != 0 {
			t.Errorf("Expected total lovers payout 0 (target didn't win), got %.2f", totalLoversPayout)
		}
		if len(winners) != 0 {
			t.Errorf("Expected no winners (target didn't win), got %d", len(winners))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Card expired (older than 24 hours)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		targetUserDiscordID := "target123"
		expiredCreatedAt := time.Now().Add(-48 * time.Hour)

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheLoversCardID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, expiredCreatedAt, expiredCreatedAt, nil, 2, guildID, cards.TheLoversCardID, nil, &targetUserDiscordID, 0.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[targetUserDiscordID] = 1000.0
		totalLoversPayout, winners, applied, err := ApplyTheLoversIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected applied true (we processed cards, just deleted the expired one)")
		}
		if totalLoversPayout != 0 {
			t.Errorf("Expected payout 0 (card expired and deleted), got %.2f", totalLoversPayout)
		}
		if len(winners) != 0 {
			t.Errorf("Expected no winners, got %d", len(winners))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Card holder is also a winner (should still get payout if their target won)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		targetUserDiscordID := "target123"
		targetWinnings := 1000.0
		expectedLoversPayout := 250.0

		loversHolderID := uint(2)
		loversHolderDiscordID := "lovers123"

		createdAt := time.Now().Add(-12 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheLoversCardID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, loversHolderID, guildID, cards.TheLoversCardID, nil, &targetUserDiscordID, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(loversHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(loversHolderID, loversHolderDiscordID, guildID, 500.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), loversHolderID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[targetUserDiscordID] = targetWinnings
		winnerDiscordIDs[loversHolderDiscordID] = 500.0
		totalLoversPayout, winners, applied, err := ApplyTheLoversIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected The Lovers to be applied")
		}
		if totalLoversPayout != expectedLoversPayout {
			t.Errorf("Expected total lovers payout %.2f, got %.2f", expectedLoversPayout, totalLoversPayout)
		}
		if len(winners) != 1 {
			t.Errorf("Expected 1 winner, got %d", len(winners))
		}
		if len(winners) > 0 && winners[0].Payout != expectedLoversPayout {
			t.Errorf("Expected winner payout %.2f, got %.2f", expectedLoversPayout, winners[0].Payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Card with no target user ID (should be skipped)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"

		createdAt := time.Now().Add(-12 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheLoversCardID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, 2, guildID, cards.TheLoversCardID, nil, nil, 0.0))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs["winner123"] = 1000.0
		totalLoversPayout, winners, applied, err := ApplyTheLoversIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected The Lovers to be applied (even if card has no target)")
		}
		if totalLoversPayout != 0 {
			t.Errorf("Expected payout 0 (no target), got %.2f", totalLoversPayout)
		}
		if len(winners) != 0 {
			t.Errorf("Expected no winners (no target), got %d", len(winners))
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

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheLoversCardID).
			WillReturnError(errors.New("db error"))

		winnerDiscordIDs := make(map[string]float64)
		_, _, applied, err := ApplyTheLoversIfApplicable(db, guildID, winnerDiscordIDs)

		if err == nil {
			t.Error("Expected error")
		}
		if applied {
			t.Error("Expected The Lovers NOT to be applied")
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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.GamblerCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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
		originalLoss := -100.0

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.GamblerCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		consumed := false
		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.GamblerCardID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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

		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.GamblerCardID).
			WillReturnError(errors.New("db error"))

		consumer := func(db *gorm.DB, u models.User, cardID uint) error {
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

func TestApplyTheDevilIfApplicable(t *testing.T) {
	t.Run("Single devil card exists, winner has it (20% diverted to pool)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		winnerDiscordID := "winner123"
		winnerWinnings := 1000.0
		expectedDiverted := 200.0

		devilHolderID := uint(2)
		devilHolderDiscordID := winnerDiscordID

		createdAt := time.Now().Add(-3 * 24 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheDevilCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, devilHolderID, guildID, cards.TheDevilCardID, nil, nil, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(devilHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(devilHolderID, devilHolderDiscordID, guildID, 500.0))

		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(guildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "guild_id", "pool"}).
				AddRow(1, guildID, 1000.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), devilHolderID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `guilds` SET `pool`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[winnerDiscordID] = winnerWinnings
		totalDiverted, divertedList, applied, err := ApplyTheDevilIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected The Devil to be applied")
		}
		if totalDiverted != expectedDiverted {
			t.Errorf("Expected total diverted %.2f, got %.2f", expectedDiverted, totalDiverted)
		}
		if len(divertedList) != 1 {
			t.Errorf("Expected 1 diverted entry, got %d", len(divertedList))
		}
		if len(divertedList) > 0 && divertedList[0].DiscordID != winnerDiscordID {
			t.Errorf("Expected diverted DiscordID '%s', got '%s'", winnerDiscordID, divertedList[0].DiscordID)
		}
		if len(divertedList) > 0 && divertedList[0].Diverted != expectedDiverted {
			t.Errorf("Expected diverted amount %.2f, got %.2f", expectedDiverted, divertedList[0].Diverted)
		}
		if winnerDiscordIDs[winnerDiscordID] != winnerWinnings-expectedDiverted {
			t.Errorf("Expected winnerDiscordIDs to be reduced to %.2f, got %.2f", winnerWinnings-expectedDiverted, winnerDiscordIDs[winnerDiscordID])
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Multiple devil cards exist for different holders (each diverts 20% of their winnings)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		winner1DiscordID := "winner123"
		winner2DiscordID := "winner456"
		winner1Winnings := 1000.0
		winner2Winnings := 800.0
		expectedDiverted1 := 200.0
		expectedDiverted2 := 160.0

		devilHolder1ID := uint(2)
		devilHolder1DiscordID := winner1DiscordID
		devilHolder2ID := uint(3)
		devilHolder2DiscordID := winner2DiscordID

		createdAt1 := time.Now().Add(-3 * 24 * time.Hour)
		createdAt2 := time.Now().Add(-3 * 24 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheDevilCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt1, createdAt1, nil, devilHolder1ID, guildID, cards.TheDevilCardID, nil, nil, 0.0).
				AddRow(2, createdAt2, createdAt2, nil, devilHolder2ID, guildID, cards.TheDevilCardID, nil, nil, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(devilHolder1ID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(devilHolder1ID, devilHolder1DiscordID, guildID, 500.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(devilHolder2ID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(devilHolder2ID, devilHolder2DiscordID, guildID, 600.0))

		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(guildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "guild_id", "pool"}).
				AddRow(1, guildID, 1000.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `guilds` SET `pool`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[winner1DiscordID] = winner1Winnings
		winnerDiscordIDs[winner2DiscordID] = winner2Winnings
		totalDiverted, divertedList, applied, err := ApplyTheDevilIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected The Devil to be applied")
		}
		expectedTotalDiverted := expectedDiverted1 + expectedDiverted2
		if totalDiverted != expectedTotalDiverted {
			t.Errorf("Expected total diverted %.2f, got %.2f", expectedTotalDiverted, totalDiverted)
		}
		if len(divertedList) != 2 {
			t.Errorf("Expected 2 diverted entries, got %d", len(divertedList))
		}
		if winnerDiscordIDs[winner1DiscordID] != winner1Winnings-expectedDiverted1 {
			t.Errorf("Expected winner1 winnings to be reduced to %.2f, got %.2f", winner1Winnings-expectedDiverted1, winnerDiscordIDs[winner1DiscordID])
		}
		if winnerDiscordIDs[winner2DiscordID] != winner2Winnings-expectedDiverted2 {
			t.Errorf("Expected winner2 winnings to be reduced to %.2f, got %.2f", winner2Winnings-expectedDiverted2, winnerDiscordIDs[winner2DiscordID])
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("No devil cards exist (no effect)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheDevilCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs["winner123"] = 500.0
		totalDiverted, divertedList, applied, err := ApplyTheDevilIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected The Devil NOT to be applied")
		}
		if totalDiverted != 0 {
			t.Errorf("Expected diverted 0, got %.2f", totalDiverted)
		}
		if len(divertedList) != 0 {
			t.Errorf("Expected no diverted entries, got %d", len(divertedList))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Winner doesn't have devil card (no effect)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		winnerDiscordID := "winner123"
		otherDevilHolderDiscordID := "devil123"

		devilHolderID := uint(2)

		createdAt := time.Now().Add(-3 * 24 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheDevilCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, devilHolderID, guildID, cards.TheDevilCardID, nil, nil, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(devilHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(devilHolderID, otherDevilHolderDiscordID, guildID, 500.0))

		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(guildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "guild_id", "pool"}).
				AddRow(1, guildID, 1000.0))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[winnerDiscordID] = 1000.0
		totalDiverted, divertedList, applied, err := ApplyTheDevilIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected The Devil NOT to be applied when winner doesn't have card and no diversion")
		}
		if totalDiverted != 0 {
			t.Errorf("Expected diverted 0 (winner doesn't have card), got %.2f", totalDiverted)
		}
		if len(divertedList) != 0 {
			t.Errorf("Expected no diverted entries, got %d", len(divertedList))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Card expired (older than 7 days)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		winnerDiscordID := "winner123"

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheDevilCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[winnerDiscordID] = 1000.0
		totalDiverted, divertedList, applied, err := ApplyTheDevilIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected The Devil NOT to be applied (card expired)")
		}
		if totalDiverted != 0 {
			t.Errorf("Expected diverted 0, got %.2f", totalDiverted)
		}
		if len(divertedList) != 0 {
			t.Errorf("Expected no diverted entries, got %d", len(divertedList))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Winner has devil card but no winnings (edge case)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		winnerDiscordID := "winner123"

		devilHolderID := uint(2)

		createdAt := time.Now().Add(-3 * 24 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheDevilCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, devilHolderID, guildID, cards.TheDevilCardID, nil, nil, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(devilHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(devilHolderID, winnerDiscordID, guildID, 500.0))

		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(guildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "guild_id", "pool"}).
				AddRow(1, guildID, 1000.0))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[winnerDiscordID] = 0.0
		totalDiverted, divertedList, applied, err := ApplyTheDevilIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected The Devil NOT to be applied when winnings are 0 (no diversion)")
		}
		if totalDiverted != 0 {
			t.Errorf("Expected diverted 0 (no winnings to divert), got %.2f", totalDiverted)
		}
		if len(divertedList) != 0 {
			t.Errorf("Expected no diverted entries, got %d", len(divertedList))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("DB Error on card query", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheDevilCardID, sqlmock.AnyArg()).
			WillReturnError(errors.New("db error"))

		winnerDiscordIDs := make(map[string]float64)
		_, _, applied, err := ApplyTheDevilIfApplicable(db, guildID, winnerDiscordIDs)

		if err == nil {
			t.Error("Expected error")
		}
		if applied {
			t.Error("Expected The Devil NOT to be applied")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("DB Error on guild query", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		winnerDiscordID := "winner123"

		devilHolderID := uint(2)

		createdAt := time.Now().Add(-3 * 24 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(guildID, cards.TheDevilCardID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount"}).
				AddRow(1, createdAt, createdAt, nil, devilHolderID, guildID, cards.TheDevilCardID, nil, nil, 0.0))

		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(devilHolderID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(devilHolderID, winnerDiscordID, guildID, 500.0))

		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(guildID, 1).
			WillReturnError(errors.New("db error"))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[winnerDiscordID] = 1000.0
		_, _, applied, err := ApplyTheDevilIfApplicable(db, guildID, winnerDiscordIDs)

		if err == nil {
			t.Error("Expected error")
		}
		if applied {
			t.Error("Expected The Devil NOT to be applied")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}

func TestApplyTheEmperorIfApplicable(t *testing.T) {
	t.Run("Emperor active, one non-holder winner (10% diverted to pool)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		emperorHolderDiscordID := "emperor123"
		winnerDiscordID := "winner456"
		winnerWinnings := 1000.0
		expectedDiverted := 100.0

		oneHourLater := time.Now().Add(1 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(guildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "guild_id", "guild_name", "bet_channel_id", "points_per_message", "starting_points", "premium_enabled", "subscribed_team", "pool", "card_draw_cost", "card_draw_cooldown_minutes", "card_drawing_enabled", "pool_drain_until", "emperor_active_until", "emperor_holder_discord_id", "total_card_draws", "last_epic_draw_at", "last_mythic_draw_at"}).
				AddRow(1, time.Now(), time.Now(), nil, guildID, "", "", 0, 0, false, nil, 500.0, 10, 60, true, nil, oneHourLater, emperorHolderDiscordID, 0, 0, 0))

		winnerUserID := uint(3)
		mock.ExpectQuery("SELECT \\* FROM `users`").
			WithArgs(winnerDiscordID, guildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
				AddRow(winnerUserID, winnerDiscordID, guildID, 600.0))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `users` SET `points`=").
			WithArgs(sqlmock.AnyArg(), winnerUserID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `guilds` SET `pool`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[winnerDiscordID] = winnerWinnings
		totalDiverted, divertedList, applied, err := ApplyTheEmperorIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected The Emperor to be applied")
		}
		if totalDiverted != expectedDiverted {
			t.Errorf("Expected total diverted %.2f, got %.2f", expectedDiverted, totalDiverted)
		}
		if len(divertedList) != 1 {
			t.Errorf("Expected 1 diverted entry, got %d", len(divertedList))
		}
		if len(divertedList) > 0 && divertedList[0].DiscordID != winnerDiscordID {
			t.Errorf("Expected diverted DiscordID '%s', got '%s'", winnerDiscordID, divertedList[0].DiscordID)
		}
		if len(divertedList) > 0 && divertedList[0].Diverted != expectedDiverted {
			t.Errorf("Expected diverted amount %.2f, got %.2f", expectedDiverted, divertedList[0].Diverted)
		}
		if winnerDiscordIDs[winnerDiscordID] != winnerWinnings-expectedDiverted {
			t.Errorf("Expected winnerDiscordIDs to be reduced to %.2f, got %.2f", winnerWinnings-expectedDiverted, winnerDiscordIDs[winnerDiscordID])
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Emperor active, holder is winner (holder excluded from diversion)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		emperorHolderDiscordID := "emperor123"
		oneHourLater := time.Now().Add(1 * time.Hour)
		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(guildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "guild_id", "guild_name", "bet_channel_id", "points_per_message", "starting_points", "premium_enabled", "subscribed_team", "pool", "card_draw_cost", "card_draw_cooldown_minutes", "card_drawing_enabled", "pool_drain_until", "emperor_active_until", "emperor_holder_discord_id", "total_card_draws", "last_epic_draw_at", "last_mythic_draw_at"}).
				AddRow(1, time.Now(), time.Now(), nil, guildID, "", "", 0, 0, false, nil, 500.0, 10, 60, true, nil, oneHourLater, emperorHolderDiscordID, 0, 0, 0))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs[emperorHolderDiscordID] = 1000.0
		totalDiverted, divertedList, applied, err := ApplyTheEmperorIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected The Emperor to be applied (even if no diversion)")
		}
		if totalDiverted != 0 {
			t.Errorf("Expected total diverted 0 (holder excluded), got %.2f", totalDiverted)
		}
		if len(divertedList) != 0 {
			t.Errorf("Expected 0 diverted entries, got %d", len(divertedList))
		}
		if winnerDiscordIDs[emperorHolderDiscordID] != 1000.0 {
			t.Errorf("Expected holder winnings unchanged at 1000, got %.2f", winnerDiscordIDs[emperorHolderDiscordID])
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("Emperor not active (nil state), no diversion", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		guildID := "guild1"
		mock.ExpectQuery("SELECT \\* FROM `guilds`").
			WithArgs(guildID, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "guild_id", "pool", "emperor_active_until", "emperor_holder_discord_id"}).
				AddRow(1, time.Now(), time.Now(), nil, guildID, 500.0, nil, nil))

		winnerDiscordIDs := make(map[string]float64)
		winnerDiscordIDs["winner456"] = 1000.0
		totalDiverted, divertedList, applied, err := ApplyTheEmperorIfApplicable(db, guildID, winnerDiscordIDs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected The Emperor NOT to be applied (no active state)")
		}
		if totalDiverted != 0 {
			t.Errorf("Expected total diverted 0, got %.2f", totalDiverted)
		}
		if len(divertedList) != 0 {
			t.Errorf("Expected 0 diverted entries, got %d", len(divertedList))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}

func TestApplyHomeFieldAdvantageIfApplicable(t *testing.T) {
	invCols := []string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount", "times_applied", "expires_at"}

	t.Run("User has active HFA (expires_at in future)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		currentPayout := 100.0
		expiresAt := time.Now().Add(1 * time.Hour)
		createdAt := time.Now()

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.HomeFieldAdvantageCardID, 1).
			WillReturnRows(sqlmock.NewRows(invCols).
				AddRow(1, createdAt, createdAt, nil, user.ID, user.GuildID, cards.HomeFieldAdvantageCardID, nil, nil, 0.0, 0, expiresAt))

		payout, applied, err := ApplyHomeFieldAdvantageIfApplicable(db, user, currentPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected Home Field Advantage to be applied")
		}
		if payout != currentPayout+15 {
			t.Errorf("Expected payout %.2f, got %.2f", currentPayout+15, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User has expired HFA (expires_at in past)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		currentPayout := 100.0
		expiresAt := time.Now().Add(-1 * time.Hour)
		createdAt := time.Now()

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.HomeFieldAdvantageCardID, 1).
			WillReturnRows(sqlmock.NewRows(invCols).
				AddRow(1, createdAt, createdAt, nil, user.ID, user.GuildID, cards.HomeFieldAdvantageCardID, nil, nil, 0.0, 0, expiresAt))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		payout, applied, err := ApplyHomeFieldAdvantageIfApplicable(db, user, currentPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected Home Field Advantage NOT to be applied (expired)")
		}
		if payout != currentPayout {
			t.Errorf("Expected payout %.2f, got %.2f", currentPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User has no HFA", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		currentPayout := 100.0

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.HomeFieldAdvantageCardID, 1).
			WillReturnRows(sqlmock.NewRows(invCols))

		payout, applied, err := ApplyHomeFieldAdvantageIfApplicable(db, user, currentPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected Home Field Advantage NOT to be applied")
		}
		if payout != currentPayout {
			t.Errorf("Expected payout %.2f, got %.2f", currentPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User has HFA not yet played (expires_at nil)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		currentPayout := 100.0
		createdAt := time.Now()

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.HomeFieldAdvantageCardID, 1).
			WillReturnRows(sqlmock.NewRows(invCols).
				AddRow(1, createdAt, createdAt, nil, user.ID, user.GuildID, cards.HomeFieldAdvantageCardID, nil, nil, 0.0, 0, nil))

		payout, applied, err := ApplyHomeFieldAdvantageIfApplicable(db, user, currentPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected Home Field Advantage NOT to be applied (not yet played)")
		}
		if payout != currentPayout {
			t.Errorf("Expected payout %.2f, got %.2f", currentPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}

func TestApplyRoughingTheKickerIfApplicable(t *testing.T) {
	invCols := []string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount", "times_applied", "expires_at"}

	t.Run("User has Roughing the Kicker in inventory (payout reduced by 15%%, card consumed)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		currentPayout := 100.0
		expectedPayout := 85.0
		createdAt := time.Now()

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.RoughingTheKickerCardID, 1).
			WillReturnRows(sqlmock.NewRows(invCols).
				AddRow(1, createdAt, createdAt, nil, user.ID, user.GuildID, cards.RoughingTheKickerCardID, nil, nil, 0.0, 0, nil))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		payout, applied, err := ApplyRoughingTheKickerIfApplicable(db, user, currentPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected Roughing the Kicker to be applied")
		}
		if payout != expectedPayout {
			t.Errorf("Expected payout %.2f (15%% reduction), got %.2f", expectedPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User has no Roughing the Kicker (payout unchanged)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		currentPayout := 100.0

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.RoughingTheKickerCardID, 1).
			WillReturnRows(sqlmock.NewRows(invCols))

		payout, applied, err := ApplyRoughingTheKickerIfApplicable(db, user, currentPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected Roughing the Kicker NOT to be applied")
		}
		if payout != currentPayout {
			t.Errorf("Expected payout %.2f, got %.2f", currentPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User has Roughing the Kicker, payout 200 -> 170", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 2, GuildID: "guild2"}
		currentPayout := 200.0
		expectedPayout := 170.0
		createdAt := time.Now()

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.RoughingTheKickerCardID, 1).
			WillReturnRows(sqlmock.NewRows(invCols).
				AddRow(2, createdAt, createdAt, nil, user.ID, user.GuildID, cards.RoughingTheKickerCardID, nil, nil, 0.0, 0, nil))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 2).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		payout, applied, err := ApplyRoughingTheKickerIfApplicable(db, user, currentPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected Roughing the Kicker to be applied")
		}
		if payout != expectedPayout {
			t.Errorf("Expected payout %.2f, got %.2f", expectedPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}

func TestApplyHeismanCampaignIfApplicable(t *testing.T) {
	invCols := []string{"id", "created_at", "updated_at", "deleted_at", "user_id", "guild_id", "card_id", "target_bet_id", "target_user_id", "bet_amount", "times_applied", "expires_at"}

	t.Run("User has Heisman Campaign in inventory (payout reduced by 15%%, card consumed)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		currentPayout := 100.0
		expectedPayout := 85.0
		createdAt := time.Now()

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.HeismanCampaignCardID, 1).
			WillReturnRows(sqlmock.NewRows(invCols).
				AddRow(1, createdAt, createdAt, nil, user.ID, user.GuildID, cards.HeismanCampaignCardID, nil, nil, 0.0, 0, nil))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 1).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		payout, applied, err := ApplyHeismanCampaignIfApplicable(db, user, currentPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected Heisman Campaign to be applied")
		}
		if payout != expectedPayout {
			t.Errorf("Expected payout %.2f (15%% reduction), got %.2f", expectedPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User has no Heisman Campaign (payout unchanged)", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 1, GuildID: "guild1"}
		currentPayout := 100.0

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.HeismanCampaignCardID, 1).
			WillReturnRows(sqlmock.NewRows(invCols))

		payout, applied, err := ApplyHeismanCampaignIfApplicable(db, user, currentPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if applied {
			t.Error("Expected Heisman Campaign NOT to be applied")
		}
		if payout != currentPayout {
			t.Errorf("Expected payout %.2f, got %.2f", currentPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})

	t.Run("User has Heisman Campaign, payout 200 -> 170", func(t *testing.T) {
		db, mock, err := newMockDB()
		if err != nil {
			t.Fatalf("Failed to create mock DB: %v", err)
		}
		defer func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()
		}()

		user := models.User{ID: 2, GuildID: "guild2"}
		currentPayout := 200.0
		expectedPayout := 170.0
		createdAt := time.Now()

		mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
			WithArgs(user.ID, user.GuildID, cards.HeismanCampaignCardID, 1).
			WillReturnRows(sqlmock.NewRows(invCols).
				AddRow(2, createdAt, createdAt, nil, user.ID, user.GuildID, cards.HeismanCampaignCardID, nil, nil, 0.0, 0, nil))

		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`=").
			WithArgs(sqlmock.AnyArg(), 2).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		payout, applied, err := ApplyHeismanCampaignIfApplicable(db, user, currentPayout)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !applied {
			t.Error("Expected Heisman Campaign to be applied")
		}
		if payout != expectedPayout {
			t.Errorf("Expected payout %.2f, got %.2f", expectedPayout, payout)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("Unmet expectations: %v", err)
		}
	})
}
