package migrations

import _ "embed"

//go:embed 0001_init.sql
var initSQL string

// InitSQL returns the versioned SQLite initialization migration.
func InitSQL() string {
	return initSQL
}
