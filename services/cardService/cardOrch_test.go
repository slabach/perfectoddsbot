package cardService

import (
	"errors"
	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"
	"testing"

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
