package searchdb

import (
	"database/sql"
	"sync"

	"github.com/mattn/go-sqlite3"
)

const driverName = "sqlite3_searchdb"

var registerOnce sync.Once

func ensureDriverRegistered() {
	registerOnce.Do(func() {
		sql.Register(driverName, &sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				_ = conn
				return nil
			},
		})
	})
}

