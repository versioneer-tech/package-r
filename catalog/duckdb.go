package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/marcboeker/go-duckdb/v2"
)

var (
	db     *sql.DB
	dbErr  error
	dbOnce sync.Once
)

func InitDuckDB() {
	dbOnce.Do(func() {
		log.Println("Initializing global DuckDB connection")
		connector, err := duckdb.NewConnector("duckdb", nil)
		if err != nil {
			dbErr = fmt.Errorf("duckdb connector error: %w", err)
			return
		}
		db = sql.OpenDB(connector)
	})
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
