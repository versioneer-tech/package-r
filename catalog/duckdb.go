package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/duckdb/duckdb-go/v2"

	appLogging "github.com/versioneer-tech/package-r/logging"
)

var (
	db     *sql.DB
	dbErr  error
	dbOnce sync.Once
)

func InitDuckDB() {
	dbOnce.Do(func() {
		started := time.Now()
		defer func() {
			elapsed := time.Since(started)
			appLogging.Timedf(
				elapsed,
				"duckdb initialize duration=%s error=%v",
				elapsed,
				dbErr,
			)
		}()
		appLogging.Debugf("duckdb initialize")
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			dbErr = fmt.Errorf("resolve DuckDB extension cache: %w", err)
			return
		}
		extensionDir := filepath.Join(cacheDir, "package-r", "duckdb-extensions")
		if err := os.MkdirAll(extensionDir, 0o700); err != nil {
			dbErr = fmt.Errorf("create DuckDB extension cache: %w", err)
			return
		}

		connector, err := duckdb.NewConnector("?extension_directory="+url.QueryEscape(extensionDir), nil)
		if err != nil {
			dbErr = fmt.Errorf("duckdb connector error: %w", err)
			return
		}
		database := sql.OpenDB(connector)
		if err := loadHTTPFS(database); err != nil {
			_ = database.Close()
			dbErr = err
			return
		}
		db = database
	})
}

func loadHTTPFS(database *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := execDuckDB(ctx, database, "load-httpfs", "LOAD httpfs"); err == nil {
		return nil
	}

	if err := execDuckDB(ctx, database, "install-httpfs", "INSTALL httpfs"); err != nil {
		return fmt.Errorf("install DuckDB httpfs extension: %w", err)
	}
	if err := execDuckDB(ctx, database, "load-httpfs", "LOAD httpfs"); err != nil {
		return fmt.Errorf("load DuckDB httpfs extension: %w", err)
	}
	return nil
}

func execDuckDB(ctx context.Context, database *sql.DB, operation, query string) error {
	started := time.Now()
	_, err := database.ExecContext(ctx, query)
	elapsed := time.Since(started)
	appLogging.Timedf(
		elapsed,
		"duckdb operation=%s duration=%s error=%v",
		operation,
		elapsed,
		err,
	)
	return err
}

func GetDuckDBConn(ctx context.Context) (*sql.Conn, error) {
	if db == nil {
		InitDuckDB()
	}
	if dbErr != nil {
		return nil, dbErr
	}

	ctxPing, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := db.PingContext(ctxPing); err != nil {
		elapsed := time.Since(started)
		appLogging.Timedf(
			elapsed,
			"duckdb ping duration=%s error=%v",
			elapsed,
			err,
		)
		appLogging.Noticef("duckdb reinitialize after ping failure error=%v", err)
		db = nil
		dbOnce = sync.Once{}
		return nil, fmt.Errorf("duckdb ping failed: %w", err)
	}
	elapsed := time.Since(started)
	appLogging.Timedf(
		elapsed,
		"duckdb ping duration=%s error=<nil>",
		elapsed,
	)

	started = time.Now()
	conn, err := db.Conn(ctx)
	elapsed = time.Since(started)
	appLogging.Timedf(
		elapsed,
		"duckdb acquire-connection duration=%s error=%v",
		elapsed,
		err,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection from DuckDB: %w", err)
	}

	return conn, nil
}
