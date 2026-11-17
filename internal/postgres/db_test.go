package postgres

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	err := godotenv.Load("../../.env")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal("could not load the repository .env; check file permissions and syntax")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgresql://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal("invalid DATABASE_URL")
	}

	admin, err := pgx.ConnectConfig(t.Context(), config.ConnConfig)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { admin.Close(context.Background()) })

	name := "labelling_test_" + rand.Text()
	quotedName := pgx.Identifier{name}.Sanitize()

	if _, err := admin.Exec(t.Context(), "CREATE DATABASE "+quotedName+" TEMPLATE template0"); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+quotedName); err != nil {
			t.Errorf("drop temporary database %s: %v", name, err)
		}
	})

	config.ConnConfig.Database = name

	db, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(db.Close)

	migration, err := os.ReadFile("../../migrations/000001_initial.up.sql")
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(t.Context(), string(migration))
	if err != nil {
		t.Fatal(err)
	}

	return db
}

func execSQL(t *testing.T, db *pgxpool.Pool, sql string, args ...any) {
	t.Helper()

	_, err := db.Exec(t.Context(), sql, args...)
	if err != nil {
		t.Fatal(err)
	}
}
