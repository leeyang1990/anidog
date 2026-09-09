//go:build !desktop

package database

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openSQLite(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(dsn), cfg)
}
