package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"syntopica-backend/internal/domain/models"
	"syntopica-backend/internal/platform/config"
	"syntopica-backend/internal/platform/logging"
)

var DB *gorm.DB

var openPostgres = connectPostgres
var runDatabaseMigrations = RunMigrations

func InitDB(cfg *config.Config) error {
	gormCfg := &gorm.Config{
		Logger:                                NewSlowLogger(200 * time.Millisecond),
		DisableForeignKeyConstraintWhenMigrating: true,
		NowFunc: func() time.Time {
			return time.Now().In(models.ShanghaiTZ)
		},
	}

	db, err := openPostgres(cfg, gormCfg)
	if err != nil {
		return err
	}

	DB = db

	// Phase 1: AutoMigrate — syncs all model tables/columns on every startup.
	// Handles ADD COLUMN and CREATE TABLE automatically. Never drops or alters.
	if err := RunAutoMigrate(db); err != nil {
		logging.Warnf("AutoMigrate warning (non-fatal): %v", err)
	}

	// Phase 2: Versioned migrations — for operations AutoMigrate can't handle:
	// extensions (pgvector), indexes, triggers, data migrations, column drops.
	if err := runDatabaseMigrations(db); err != nil {
		return fmt.Errorf("run database migrations: %w", err)
	}

	logging.Infof("Database initialized successfully")
	return nil
}

