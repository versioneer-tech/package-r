package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/duckdb/duckdb-go/v2"
)

var (
	db     *sql.DB
	dbErr  error
	dbOnce sync.Once
)

func InitDuckDB() {
	dbOnce.Do(func() {
		log.Println("Initializing global DuckDB connection")
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

	if _, err := database.ExecContext(ctx, "LOAD httpfs"); err == nil {
		return nil
	}

	log.Println("Installing DuckDB httpfs extension")
	if _, err := database.ExecContext(ctx, "INSTALL httpfs"); err != nil {
		return fmt.Errorf("install DuckDB httpfs extension: %w", err)
	}
	if _, err := database.ExecContext(ctx, "LOAD httpfs"); err != nil {
		return fmt.Errorf("load DuckDB httpfs extension: %w", err)
	}
	return nil
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
	if err := db.PingContext(ctxPing); err != nil {
		log.Println("Reinitializing DuckDB due to ping failure:", err)
		db = nil
		dbOnce = sync.Once{}
		return nil, fmt.Errorf("duckdb ping failed: %w", err)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection from DuckDB: %w", err)
	}

	return conn, nil
}
