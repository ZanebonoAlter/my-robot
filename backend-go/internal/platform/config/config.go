package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	CORS     CORSConfig
	Log      LogConfig
	Tracing  TracingConfig
}

type LogConfig struct {
	Level string        `mapstructure:"level"`
	File  LogFileConfig `mapstructure:"file"`
}

type LogFileConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Path       string `mapstructure:"path"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Compress   bool   `mapstructure:"compress"`
}

type ServerConfig struct {
	Port string
	Mode string // debug, release, test
}

type DatabaseConfig struct {
	Driver   string
	DSN      string
	Postgres PostgresConfig

	// AllowDestructiveMigrations controls whether destructive migrations
	// (TRUNCATE/DROP) execute. Defaults to false (production-safe). Set via
	// env MIGRATIONS_ALLOW_DESTRUCTIVE=1 for dev/test environments that need
	// historical data cleanup. See db-migration-safety capability.
	AllowDestructiveMigrations bool
}

type PostgresConfig struct {
	MaxIdleConns           int `mapstructure:"max_idle_conns"`
	MaxOpenConns           int `mapstructure:"max_open_conns"`
	ConnMaxLifetimeMinutes int `mapstructure:"conn_max_lifetime_minutes"`
	ConnMaxIdleTimeMinutes int `mapstructure:"conn_max_idle_time_minutes"`
}

type CORSConfig struct {
	Origins      []string
	Methods      []string
	AllowHeaders []string
}

type TracingConfig struct {
	SampleRatio    float64 `mapstructure:"sample_ratio"`
	InstrumentGORM bool    `mapstructure:"instrument_gorm"`
	InstrumentHTTP bool    `mapstructure:"instrument_http"`
}

var AppConfig *Config

func LoadConfig(configPath string) error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configPath)
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")

	// Set defaults
	viper.SetDefault("server.port", "5000")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("database.driver", "postgres")
	viper.SetDefault("database.dsn", "host=127.0.0.1 user=postgres password=postgres dbname=syntopica port=5432 sslmode=disable TimeZone=Asia/Shanghai")
	viper.SetDefault("database.postgres.max_idle_conns", 5)
	viper.SetDefault("database.postgres.max_open_conns", 25)
	viper.SetDefault("database.postgres.conn_max_lifetime_minutes", 60)
	viper.SetDefault("database.postgres.conn_max_idle_time_minutes", 10)
	viper.SetDefault("cors.origins", []string{"http://localhost:3000", "http://localhost:3000"})
	viper.SetDefault("cors.methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	viper.SetDefault("cors.allow_headers", []string{"Content-Type", "Authorization"})

	viper.SetDefault("log.level", "debug")
	viper.SetDefault("log.file.enabled", true)
	viper.SetDefault("log.file.path", "logs/app.log")
	viper.SetDefault("log.file.max_size_mb", 50)
	viper.SetDefault("log.file.max_backups", 30)
	viper.SetDefault("log.file.max_age_days", 30)
	viper.SetDefault("log.file.compress", true)

	viper.SetDefault("tracing.sample_ratio", 1.0)
	viper.SetDefault("tracing.instrument_gorm", true)
	viper.SetDefault("tracing.instrument_http", true)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
		fmt.Println("Config file not found, using defaults")
	}

	AppConfig = &Config{}

	if err := viper.Unmarshal(AppConfig); err != nil {
		return err
	}

	applyEnvOverrides(AppConfig)

	return nil
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	if value := strings.TrimSpace(os.Getenv("SERVER_PORT")); value != "" {
		cfg.Server.Port = value
	}

	if value := strings.TrimSpace(os.Getenv("SERVER_MODE")); value != "" {
		cfg.Server.Mode = value
	}

	if value := strings.TrimSpace(os.Getenv("DATABASE_DRIVER")); value != "" {
		cfg.Database.Driver = value
	}

	if value := strings.TrimSpace(os.Getenv("DATABASE_DSN")); value != "" {
		cfg.Database.DSN = value
	}

	if value := strings.TrimSpace(os.Getenv("CORS_ORIGINS")); value != "" {
		cfg.CORS.Origins = splitCommaSeparated(value)
	}

	// MIGRATIONS_ALLOW_DESTRUCTIVE=1 enables destructive migrations (TRUNCATE/DROP).
	// Defaults to false: production never sets this, refusing destructive migrations.
	cfg.Database.AllowDestructiveMigrations = os.Getenv("MIGRATIONS_ALLOW_DESTRUCTIVE") == "1"

	// Tracing (defaults via viper SetDefault above; env overrides when set)
	if v := strings.TrimSpace(os.Getenv("TRACE_SAMPLE_RATIO")); v != "" {
		if r, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Tracing.SampleRatio = r
		}
	}
	if v := os.Getenv("TRACE_INSTRUMENT_GORM"); v != "" {
		cfg.Tracing.InstrumentGORM = v != "0"
	}
	if v := os.Getenv("TRACE_INSTRUMENT_HTTP"); v != "" {
		cfg.Tracing.InstrumentHTTP = v != "0"
	}
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
