package sqlite

import "errors"

var (
	// ErrMigration is returned when the schema migration fails.
	ErrMigration = errors.New("sqlite: schema migration failed")
)
