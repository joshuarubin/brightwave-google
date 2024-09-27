package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var Schema string

const (
	SQLITE_INSERT = sqlite3.SQLITE_INSERT //nolint:revive,stylecheck
	SQLITE_DELETE = sqlite3.SQLITE_DELETE //nolint:revive,stylecheck
	SQLITE_UPDATE = sqlite3.SQLITE_UPDATE //nolint:revive,stylecheck
)

type UpdateHook func(operation int, dbName string, tableName string, rowID int64)

func Register(name string, callback UpdateHook) {
	sql.Register(name, &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			slog.Info("registering update hook", "name", name)
			conn.RegisterUpdateHook(callback)
			return nil
		},
	})
}

type DB struct {
	sync.RWMutex
	SQL *sql.DB
	*Queries
}

var num atomic.Uint32

func Init(ctx context.Context, file string, callback UpdateHook) (*DB, error) {
	name := fmt.Sprintf("sqlite3_%d", num.Add(1))
	Register(name, callback)

	// TODO(jrubin) validate DBFile
	db, err := sql.Open(name, fmt.Sprintf("file:%s?_foreign_keys=1&_journal=wal&cache=shared&_mutex=full&_locking_mode=NORMAL&mode=rwc&_synchronous=NORMAL&_txlock=immediate&_cache_size=-10000", file))
	if err != nil {
		return nil, err
	}

	// create tables
	if _, err = db.ExecContext(ctx, Schema); err != nil {
		return nil, err
	}

	queries, err := Prepare(ctx, db)
	if err != nil {
		return nil, err
	}

	return &DB{
		SQL:     db,
		Queries: queries,
	}, nil
}
