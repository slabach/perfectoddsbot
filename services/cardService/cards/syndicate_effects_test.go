package cards

import (
	"perfectOddsBot/models"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPlaceDiceSecondCopyDoesNotConsume(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	ok, err := PlaceDice(db, models.User{ID: 1, GuildID: "g"}, models.BetEntry{ID: 9, BetID: 3}, 1.1)
	if err != nil || ok {
		t.Fatalf("second die must fizzle without consuming, ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPlaceFixSecondCopyDoesNotConsume(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	ok, err := PlaceFix(db, models.User{ID: 1, GuildID: "g"}, models.BetEntry{ID: 9, BetID: 3})
	if err != nil || ok {
		t.Fatalf("second fix must fizzle, ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPlaceKnucklesBothSidesFizzles(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `bet_entries`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	ok, err := PlaceKnuckles(db, models.User{ID: 1, GuildID: "g"}, models.BetEntry{ID: 9, BetID: 3, Option: 1}, "target")
	if err != nil || ok {
		t.Fatalf("both sides must not be a candidate, ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPlaceSpeakeasyRejectsThreeLegs(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `parlay_entries`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	ok, err := PlaceSpeakeasy(db, models.User{ID: 1, GuildID: "g"}, models.Parlay{ID: 4, Status: "pending"})
	if err != nil || ok {
		t.Fatalf("3-leg parlay must be rejected, ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenLineNilSpreadDoesNotConsume(t *testing.T) {
	db, mock := newMockDB(t)
	ok, err := OpenLineEntry(db, models.User{ID: 1, GuildID: "g"}, models.BetEntry{ID: 9}, nil)
	if err != nil || ok {
		t.Fatalf("nil opening line must fizzle, ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShaveEntryMovesSnapshotAndConsumes(t *testing.T) {
	db, mock := newMockDB(t)
	spread := 0.0
	expectWrite(mock, "UPDATE `bet_entries`")
	expectWrite(mock, "UPDATE `parlay_entries`")
	expectConsumeHeld(mock, PointShaverCardID)

	err := ShaveEntry(db, models.User{ID: 1, GuildID: "g"}, models.BetEntry{ID: 9, BetID: 3, Option: 1, Spread: &spread})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestShaveStacksAnotherPointAndAHalf(t *testing.T) {
	db, mock := newMockDB(t)
	spread := 1.5
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `bet_entries` SET `spread`=\\?").
		WithArgs(3.0, sqlmock.AnyArg(), 9).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectWrite(mock, "UPDATE `parlay_entries`")
	expectConsumeHeld(mock, PointShaverCardID)

	err := ShaveEntry(db, models.User{ID: 1, GuildID: "g"}, models.BetEntry{ID: 9, BetID: 3, Option: 1, Spread: &spread})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCookedBooksCreatesExpiringRow(t *testing.T) {
	db, mock := newMockDB(t)
	expectConsumeHeld(mock, CookedBooksCardID)
	expectWrite(mock, "INSERT INTO `user_inventories`")

	if err := StartCookedBooks(db, models.User{ID: 1, GuildID: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestProtectedByCookedBooksRequiresUnexpiredRow(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	on, err := ProtectedByCookedBooks(db, 1, "g")
	if err != nil || on {
		t.Fatalf("expired or missing books must not protect, on=%v err=%v", on, err)
	}
}

func TestHasHoochCurseIgnoresHeldCopy(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories` WHERE .*target_user_id IS NOT NULL").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	on, err := HasHoochCurse(db, 1, "g")
	if err != nil || !on {
		t.Fatalf("curse row should count, on=%v err=%v", on, err)
	}
}

func TestConcreteShoesActive(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	on, err := ConcreteShoesActive(db, 1, "g")
	if err != nil || !on {
		t.Fatalf("shoes on=%v err=%v", on, err)
	}
}

func expectWrite(mock sqlmock.Sqlmock, query string) {
	mock.ExpectBegin()
	mock.ExpectExec(query).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func TestStreetTaxFizzlesWhenTheTopThreeCannotPay(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `users`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(1, "drawer", "g", 50))
	mock.ExpectQuery("SELECT \\* FROM `users`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(1, "drawer", "g", 50))
	mock.ExpectCommit()

	result, err := handleStreetTax(nil, db, "drawer", "g")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.PointsDelta != 0 {
		t.Fatalf("expected a fizzle, got %#v", result)
	}
}

func TestLeveragedBuyoutFizzlesWithoutACommon(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT \\* FROM `users`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id"}).
			AddRow(1, "caster", "g"))
	mock.ExpectQuery("SELECT \\* FROM `users`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id"}).
			AddRow(2, "target", "g"))
	mock.ExpectQuery("SELECT user_inventories.id, cards.name FROM `user_inventories`.*target_entry_id IS NULL.*target_user_id IS NULL.*expires_at IS NULL.*bet_amount = 0.*times_applied = 0").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	_, _, ok, err := LeveragedBuyoutSwap(db, models.User{ID: 1, GuildID: "g"}, models.User{ID: 2, GuildID: "g"})
	if err != nil || ok {
		t.Fatalf("missing common must fizzle without a swap, ok=%v err=%v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectConsumeHeld(mock sqlmock.Sqlmock, cardID uint) {
	mock.ExpectQuery("SELECT \\* FROM `user_inventories`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "guild_id", "card_id"}).
			AddRow(8, 1, "g", cardID))
	expectWrite(mock, "UPDATE `user_inventories` SET `deleted_at`")
}

func TestRefundEntryReturnsStakeAndDeletesEntry(t *testing.T) {
	db, mock := newMockDB(t)
	expectWrite(mock, "UPDATE `users`")
	expectWrite(mock, "UPDATE `bet_entries` SET `deleted_at`")
	expectWrite(mock, "UPDATE `user_inventories` SET `deleted_at`")
	expectConsumeHeld(mock, BackroomWiretapCardID)

	err := RefundEntry(db, models.User{ID: 1, GuildID: "g"}, models.BetEntry{ID: 9, Amount: 40})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
