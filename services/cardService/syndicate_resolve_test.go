package cardService

import (
	"testing"
	"time"

	"perfectOddsBot/models"
	"perfectOddsBot/services/cardService/cards"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
)

func TestApplyLoadedDiceConsumesInvalidRollWithoutChangingPayout(t *testing.T) {
	db, mock, err := newMockDB()
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "bet_amount", "target_entry_id"}).
			AddRow(4, 1, "g", cards.LoadedDiceCardID, 0, 9))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, applied, err := ApplyLoadedDice(db, models.User{ID: 1, GuildID: "g"}, 9, 100)
	if err != nil || applied || got != 100 {
		t.Fatalf("invalid roll must leave the payout, got=%v applied=%v err=%v", got, applied, err)
	}
}

func TestApplyLoadedDiceMultipliesOnce(t *testing.T) {
	db, mock, err := newMockDB()
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "bet_amount", "target_entry_id"}).
			AddRow(4, 1, "g", cards.LoadedDiceCardID, 1.25, 9))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, applied, err := ApplyLoadedDice(db, models.User{ID: 1, GuildID: "g"}, 9, 100)
	if err != nil || !applied || got != 125 {
		t.Fatalf("got=%v applied=%v err=%v", got, applied, err)
	}
}

func TestApplyFixMoneylineDoesNotQuery(t *testing.T) {
	db, mock, err := newMockDB()
	if err != nil {
		t.Fatal(err)
	}
	push, err := ApplyFixIsIn(db, models.User{ID: 1, GuildID: "g"}, models.BetEntry{ID: 9, Option: 1}, models.Bet{}, 0)
	if err != nil || push {
		t.Fatalf("moneyline must be ignored, push=%v err=%v", push, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyFixPushAndMissConsumeTheRow(t *testing.T) {
	spread := 3.5
	entry := models.BetEntry{ID: 9, Option: 1, Spread: &spread}
	bet := models.Bet{Spread: &spread}

	for _, tc := range []struct {
		name      string
		scoreDiff int
		push      bool
	}{
		{"miss 0.5 pushes", -4, true},
		{"miss 3.5 does not push", -7, false},
		{"a cover does not push", 10, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := newMockDB()
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
				WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "target_entry_id"}).
					AddRow(4, 1, "g", cards.TheFixIsInCardID, 9))
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			push, err := ApplyFixIsIn(db, models.User{ID: 1, GuildID: "g"}, entry, bet, tc.scoreDiff)
			if err != nil || push != tc.push {
				t.Fatalf("push=%v want %v err=%v", push, tc.push, err)
			}
		})
	}
}

func TestConsumeFixForBetDeletesWithoutRefund(t *testing.T) {
	db, mock, err := newMockDB()
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_inventories` SET `deleted_at`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := ConsumeFixForBet(db, 12); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDivertBagmanExpiryKeepsThePoolWhole(t *testing.T) {
	db, _, err := newMockDB()
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Minute)
	holder := "holder"
	guild := &models.Guild{GuildID: "g", BagmanActiveUntil: &past, BagmanHolderDiscordID: &holder}
	pool, cut, err := DivertBagman(db, guild, 100)
	if err != nil || cut != 0 || pool != 100 {
		t.Fatalf("pool=%v cut=%v err=%v", pool, cut, err)
	}
}

func TestDivertBagmanPaysFifteen(t *testing.T) {
	db, mock, err := newMockDB()
	if err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(time.Hour)
	holder := "holder"
	guild := &models.Guild{GuildID: "g", BagmanActiveUntil: &until, BagmanHolderDiscordID: &holder}
	mock.ExpectQuery("SELECT \\* FROM `users`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(2, "holder", "g", 10))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET `points`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	pool, cut, err := DivertBagman(db, guild, 100)
	if err != nil || cut != 15 || pool != 85 {
		t.Fatalf("pool=%v cut=%v err=%v", pool, cut, err)
	}
}

func TestApplyDonLossBillZeroDoesNotQuery(t *testing.T) {
	db, mock, err := newMockDB()
	if err != nil {
		t.Fatal(err)
	}
	bill, err := ApplyDonLossBill(db, models.User{ID: 1, GuildID: "g", DiscordID: "loser"}, 0)
	if err != nil || bill != 0 {
		t.Fatalf("bill=%v err=%v", bill, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyDonLossBillChargesThePartnerWhenTheCasterLoses(t *testing.T) {
	db, mock, err := newMockDB()
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
		WithArgs(uint(1), "g", cards.TheDonsFavorCardID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id", "target_user_id", "expires_at"}).
			AddRow(5, 1, "g", cards.TheDonsFavorCardID, "partner", expires))
	mock.ExpectQuery("SELECT \\* FROM `users`").
		WithArgs("partner", "g", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(2, "partner", "g", 1000.0))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WithArgs(2, "g", cards.CookedBooksCardID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET `points`").
		WithArgs(25.0, 2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET `points`").
		WithArgs(25.0, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	bill, err := ApplyDonLossBill(db, models.User{ID: 1, GuildID: "g", DiscordID: "caster"}, 100)
	if err != nil || bill != 25 {
		t.Fatalf("bill=%v err=%v", bill, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestParlaySpeakeasyCount(t *testing.T) {
	db, mock, err := newMockDB()
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	on, err := ParlayHasSpeakeasy(db, 4)
	if err != nil || on {
		t.Fatalf("on=%v err=%v", on, err)
	}
}

var _ = gorm.ErrRecordNotFound
