package database

import (
	"testing"

	"syntopica-backend/internal/platform/config"
)

func TestInitDBConnectsToPostgres(t *testing.T) {
	if err := config.LoadConfig("../../configs"); err != nil {
		t.Fatalf("load config: %v", err)
	}

	if err := InitDB(config.AppConfig); err != nil {
		t.Fatalf("InitDB returned error: %v", err)
	}

	if DB == nil {
		t.Fatal("expected global DB to be initialized")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		t.Fatalf("get underlying db: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
}
