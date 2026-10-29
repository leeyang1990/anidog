package database

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// migrateSchema owns the schema transition before application workers start.
// SQLite's table rebuilds temporarily reference the old table, so self-FKs
// cannot remain enabled on the migration connection. Other connections retain
// enforcement; all rebuilt data is checked before the single transaction commits.
func migrateSchema(db *gorm.DB, models ...interface{}) error {
	if db.Dialector.Name() != "sqlite" {
		return db.AutoMigrate(models...)
	}
	return db.Connection(func(conn *gorm.DB) (migrationErr error) {
		if err := conn.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
			return err
		}
		defer func() {
			restoreErr := conn.Exec("PRAGMA foreign_keys = ON").Error
			if restoreErr == nil {
				var enabled int
				restoreErr = conn.Raw("PRAGMA foreign_keys").Scan(&enabled).Error
				if restoreErr == nil && enabled != 1 {
					restoreErr = fmt.Errorf("SQLite migration connection did not restore foreign keys")
				}
			}
			migrationErr = errors.Join(migrationErr, restoreErr)
		}()
		return conn.Transaction(func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(models...); err != nil {
				return err
			}
			rows, err := tx.Raw("PRAGMA foreign_key_check").Rows()
			if err != nil {
				return err
			}
			defer rows.Close()
			if rows.Next() {
				return fmt.Errorf("SQLite schema migration would retain invalid foreign-key references")
			}
			return rows.Err()
		})
	})
}
