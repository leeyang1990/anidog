//go:build desktop

package database

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 桌面构建使用纯 Go SQLite，避免内嵌 BT 的 SQLite 存储与
// mattn/go-sqlite3 的 C amalgamation 在最终应用中产生重复符号。
func openSQLite(dsn string, cfg *gorm.Config) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(dsn), cfg)
}
