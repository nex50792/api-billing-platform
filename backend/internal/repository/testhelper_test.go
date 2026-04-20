package repository_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func buildDSN() string {
	host := getEnvOrDefault("DB_HOST", "localhost")
	port := getEnvOrDefault("DB_PORT", "5436")
	user := getEnvOrDefault("DB_USER", "postgres")
	password := getEnvOrDefault("DB_PASSWORD", "postgres")
	dbname := getEnvOrDefault("DB_NAME", "api_billing_test")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
}

func TestMain(m *testing.M) {
	db, err := sql.Open("pgx", buildDSN())
	if err == nil {
		if pingErr := db.Ping(); pingErr == nil {
			_, thisFile, _, _ := runtime.Caller(0)
			migrationDir := filepath.Join(filepath.Dir(thisFile), "../../migrations")
			db.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
			if data, err := os.ReadFile(filepath.Join(migrationDir, "000001_init.up.sql")); err == nil {
				db.Exec(string(data))
			}
			if data, err := os.ReadFile(filepath.Join(migrationDir, "test_seed.sql")); err == nil {
				db.Exec(string(data))
			}
		}
		db.Close()
	}
	os.Exit(m.Run())
}

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := buildDSN()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("test DB not available, skipping: %v", err)
	}
	return db
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
