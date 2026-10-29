//go:build !desktop

package database

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openSQLite(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(withSQLiteOption(dsn, "_foreign_keys=on")), cfg)
}
