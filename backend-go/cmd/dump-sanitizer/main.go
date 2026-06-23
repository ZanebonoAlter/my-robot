// Command dump-sanitizer exports recent business data from a running Syntopica
// database, applies conservative sanitization, and writes a portable seed.sql
// suitable for the public read-only demo instance.
//
// Usage (from the backend-go/ directory, so configs/config.yaml resolves):
//
//	cd backend-go
//	go run ./cmd/dump-sanitizer
//
// Environment:
//
//	DATABASE_DSN    PostgreSQL DSN (overrides configs/config.yaml)
//	EXPORT_DAYS     recent-data window in days (default 30)
//	SEED_OUT        output file path (default ../demo/seed/seed.sql)
//
// The schema (tables, columns, indexes, triggers) is NOT exported — the target
// database builds it itself via AutoMigrate + versioned migrations at startup.
// Only INSERT data + sequence resets are emitted.
package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "dump-sanitizer: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	outPath := flag.String("out", envOr("SEED_OUT", filepath.Join("..", "demo", "seed", "seed.sql")), "output seed.sql path")
	dsnFlag := flag.String("dsn", "", "PostgreSQL DSN (overrides config + DATABASE_DSN)")
	days := flag.Int("days", envIntOr("EXPORT_DAYS", 30), "recent-data window in days")
	flag.Parse()

	dsn := *dsnFlag
	if dsn == "" {
		dsn = resolveDSN()
	}
	if dsn == "" {
		return fmt.Errorf("no DSN: set DATABASE_DSN or run from backend-go/ (configs/config.yaml)")
	}

	fmt.Printf("Connecting to source database (DSN host masked)...\n")
	db, err := openDB(dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(*outPath), err)
	}

	f, err := os.Create(*outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", *outPath, err)
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	totalRows, totalBytes, err := writeSeed(w, db, *days)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	fmt.Printf("\nDone. %d rows, %.1f KB → %s\n", totalRows, float64(totalBytes)/1024.0, *outPath)
	fmt.Printf("Window: last %d days. Re-run after schema changes or to refresh data.\n", *days)
	return nil
}

// writeSeed emits the file header then each table's INSERT block. Returns total
// rows and bytes written.
func writeSeed(w *bufio.Writer, db *sql.DB, days int) (int, int, error) {
	totalRows := 0
	totalBytes := 0

	// File header / hygiene.
	hdr := fmt.Sprintf(
		"-- Syntopica demo seed (sanitized).\n"+
			"-- Generated: %s\n"+
			"-- Window: last %d days. Schema is built by the app at startup (AutoMigrate + migrations);\n"+
			"-- this file only contains INSERT data and sequence resets.\n"+
			"-- DO NOT edit by hand — regenerate with: cd backend-go && go run ./cmd/dump-sanitizer\n"+
			"-- All ai_providers.api_key/base_url cleared; reading_behaviors.session_id hashed;\n"+
			"-- all pgvector embedding columns set to NULL; ai_call_logs/schema_migrations excluded.\n\n",
		time.Now().Format("2006-01-02 15:04:05"), days,
	)
	if _, err := w.WriteString(hdr); err != nil {
		return 0, 0, err
	}
	totalBytes += len(hdr)

	specs := exportSpecs()
	for _, spec := range specs {
		fmt.Printf("  %-34s ", spec.Table+"...")
		n, wrote, err := exportTable(w, db, spec, days)
		if err != nil {
			fmt.Println("FAIL")
			return totalRows, totalBytes, fmt.Errorf("table %s: %w", spec.Table, err)
		}
		totalRows += n
		totalBytes += wrote
		fmt.Printf("%d rows\n", n)
	}
	return totalRows, totalBytes, nil
}

// exportTable reads one table and writes its INSERT block + sequence reset.
// Returns (rows exported, bytes written).
func exportTable(w *bufio.Writer, db *sql.DB, spec ExportSpec, days int) (int, int, error) {
	selectSQL := buildSelect(spec, days)
	rows, err := db.Query(selectSQL)
	if err != nil {
		return 0, 0, fmt.Errorf("query: %w\n  SQL: %s", err, selectSQL)
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return 0, 0, fmt.Errorf("columns: %w", err)
	}

	header := fmt.Sprintf("\n-- %s\n", spec.Table)
	if _, err := w.WriteString(header); err != nil {
		return 0, 0, err
	}
	written := len(header)
	count := 0

	const batchSize = 500
	var batch strings.Builder
	endInsert := func() {
		if spec.ConflictClause != "" {
			batch.WriteByte(' ')
			batch.WriteString(spec.ConflictClause)
		}
		batch.WriteString(";\n")
	}
	flushBatch := func() error {
		if batch.Len() == 0 {
			return nil
		}
		s := batch.String()
		if _, err := w.WriteString(s); err != nil {
			return err
		}
		written += len(s)
		batch.Reset()
		return nil
	}

	for rows.Next() {
		raw := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return count, written, fmt.Errorf("scan: %w", err)
		}

		// Compose the VALUES tuple for this row.
		var vals strings.Builder
		vals.WriteByte('(')
		for i, c := range cols {
			if i > 0 {
				vals.WriteByte(',')
			}
			san := spec.Sanitizers[c]
			if raw[i].Valid {
				s := raw[i].String
				if san != nil {
					s = san(s)
				}
				vals.WriteString(quoteString(s))
			} else {
				vals.WriteString("NULL")
			}
		}
		vals.WriteByte(')')

		if count%batchSize == 0 {
			// Start a new INSERT statement.
			if err := flushBatch(); err != nil {
				return count, written, err
			}
			fmt.Fprintf(&batch, "INSERT INTO %s (%s) VALUES\n%s", spec.Table, strings.Join(spec.Columns, ","), vals.String())
		} else {
			fmt.Fprintf(&batch, ",\n%s", vals.String())
		}

		// Closing ';' every batchSize or at end-of-table is added when flushing
		// the next batch starts (above) or after the loop (below).
		count++
		// Mark that the current batch needs a terminator before the next one.
		if count%batchSize == 0 {
			endInsert()
		}
	}
	if err := rows.Err(); err != nil {
		return count, written, fmt.Errorf("rows: %w", err)
	}

	// Terminate the final partial batch.
	if count%batchSize != 0 && batch.Len() > 0 {
		endInsert()
	}
	if err := flushBatch(); err != nil {
		return count, written, err
	}

	// Sequence reset (skip composite-PK tables).
	if !spec.NoSequence && count > 0 {
		seq := fmt.Sprintf(
			"SELECT setval(pg_get_serial_sequence('%s','id'), COALESCE(MAX(id),0)+1, false) FROM %s;\n",
			spec.Table, spec.Table,
		)
		if _, err := w.WriteString(seq); err != nil {
			return count, written, err
		}
		written += len(seq)
	}

	if count == 0 {
		empty := fmt.Sprintf("-- (no rows matched for %s in the last %d days)\n", spec.Table, days)
		if _, err := w.WriteString(empty); err != nil {
			return count, written, err
		}
		written += len(empty)
	}

	return count, written, nil
}

// buildSelect assembles the SELECT statement: vector columns are projected as
// NULL::vector, and the :days placeholder in Where is substituted.
func buildSelect(spec ExportSpec, days int) string {
	proj := make([]string, len(spec.Columns))
	for i, c := range spec.Columns {
		if spec.VectorColumns[c] {
			proj[i] = fmt.Sprintf("NULL::vector AS %s", c)
		} else {
			proj[i] = c
		}
	}
	q := fmt.Sprintf("SELECT %s FROM %s", strings.Join(proj, ","), spec.Table)
	if spec.Where != "" {
		q += " WHERE " + strings.ReplaceAll(spec.Where, ":days", strconv.Itoa(days))
	}
	return q
}

// --- DB connection -----------------------------------------------------------

func openDB(dsn string) (*sql.DB, error) {
	gdb, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return gdb.DB()
}

// resolveDSN reads configs/config.yaml (when run from backend-go/) or the
// DATABASE_DSN environment variable.
func resolveDSN() string {
	if v := strings.TrimSpace(os.Getenv("DATABASE_DSN")); v != "" {
		return v
	}
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("configs")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		return ""
	}
	return viper.GetString("database.dsn")
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
