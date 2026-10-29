package database

import "strings"

// Driver adapters own their connection options. Foreign keys must be enabled
// on every connection, including replacements opened after migrations.
func withSQLiteOption(dsn, option string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + option
}
