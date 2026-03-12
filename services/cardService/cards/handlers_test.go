package cards

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	return gormDB, mock
}

func TestHandleNuke_AtomicUpdate(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery("SELECT \\* FROM `users`").
		WithArgs("guild1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(1, "user1", "guild1", 100.0))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET .* WHERE guild_id = \\? AND `users`.`deleted_at` IS NULL").
		WithArgs(sqlmock.AnyArg(), "guild1").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT \\* FROM `guilds` WHERE guild_id = \\? AND `guilds`.`deleted_at` IS NULL ORDER BY `guilds`.`id` LIMIT \\? FOR UPDATE").
		WithArgs("guild1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "guild_id", "pool"}).
			AddRow(1, "guild1", 1000.0))

	if _, err := handleNuke(nil, db, "user1", "guild1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestHandleMajorGlitch_AtomicUpdate(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `users`").
		WithArgs("guild1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE \\(guild_id = \\? AND discord_id != \\?\\) AND `users`.`deleted_at` IS NULL").
		WithArgs("guild1", "drawer1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(2, "target1", "guild1", 120.0))
	mock.ExpectQuery("SELECT \\* FROM `users` WHERE `users`.`id` = \\? AND `users`.`deleted_at` IS NULL ORDER BY `users`.`id` LIMIT \\? FOR UPDATE").
		WithArgs(2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(2, "target1", "guild1", 120.0))
	mock.ExpectExec("UPDATE `users` SET .* WHERE `users`.`deleted_at` IS NULL AND `id` = \\?").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 2).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO `card_play_histories` .*").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if _, err := handleMajorGlitch(nil, db, "drawer1", "guild1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestHandleStimulusCheck_AtomicUpdate(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `users`").
		WithArgs("guild1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `users`").
		WithArgs("guild1", "drawer1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery("SELECT \\* FROM `users` WHERE \\(guild_id = \\? AND discord_id != \\?\\) AND `users`.`deleted_at` IS NULL").
		WithArgs("guild1", "drawer1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(2, "u2", "guild1", 100.0).
			AddRow(3, "u3", "guild1", 200.0))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET .* WHERE \\(guild_id = \\? AND discord_id != \\?\\) AND `users`.`deleted_at` IS NULL").
		WithArgs(50.0, sqlmock.AnyArg(), "guild1", "drawer1").
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `card_play_histories` .*").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `card_play_histories` .*").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if _, err := handleStimulusCheck(nil, db, "drawer1", "guild1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestHandleTipJar_Transaction(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery("SELECT .* FROM `users` .*FOR UPDATE").
		WithArgs("drawer1", "guild1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(1, "drawer1", "guild1", 100.0))

	mock.ExpectQuery("SELECT .* FROM `users` .*FOR UPDATE").
		WithArgs("guild1", 100.0, 100.0, 1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(2, "top1", "guild1", 200.0))

	if _, err := handleTipJar(nil, db, "drawer1", "guild1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestHandleBlueShell_Transaction(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `users` .*FOR UPDATE").
		WithArgs("guild1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points", "username"}).
			AddRow(1, "first1", "guild1", 600.0, nil))

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WithArgs(1, "guild1", TheMoonCardID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .* FROM `user_inventories` WHERE .*card_id = \\?.*ORDER BY created_at DESC.*LIMIT \\?").
		WithArgs(1, "guild1", RedshirtCardID, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WithArgs(1, "guild1", ShieldCardID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectCommit()

	if _, err := handleBlueShell(nil, db, "drawer1", "guild1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestExecutePickpocketSteal_LocksAndUpdates(t *testing.T) {
	db, mock := newMockDB(t)

	mock.ExpectQuery("SELECT .* FROM `users` .*FOR UPDATE").
		WithArgs("drawer1", "guild1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(1, "drawer1", "guild1", 100.0))

	mock.ExpectQuery("SELECT .* FROM `users` .*FOR UPDATE").
		WithArgs("target1", "guild1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "discord_id", "guild_id", "points"}).
			AddRow(2, "target1", "guild1", 80.0))

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WithArgs(2, "guild1", TheMoonCardID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .* FROM `user_inventories` WHERE .*card_id = \\?.*ORDER BY created_at DESC.*LIMIT \\?").
		WithArgs(2, "guild1", RedshirtCardID, 1).
		WillReturnError(gorm.ErrRecordNotFound)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_inventories`").
		WithArgs(2, "guild1", ShieldCardID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET .* WHERE `users`.`deleted_at` IS NULL AND `id` = \\?").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `users` SET .* WHERE `users`.`deleted_at` IS NULL AND `id` = \\?").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 2).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT \\* FROM `user_inventories` WHERE \\(user_id = \\? AND guild_id = \\? AND card_id = \\? AND deleted_at IS NULL\\) AND `user_inventories`.`deleted_at` IS NULL").
		WithArgs(2, "guild1", BountyHunterCardID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, err := ExecutePickpocketSteal(db, "drawer1", "target1", "guild1", 50.0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
