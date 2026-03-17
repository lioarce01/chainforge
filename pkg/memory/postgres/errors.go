package postgres

import "errors"

var (
	// ErrNoDSN is returned by New when no DSN is provided.
	ErrNoDSN = errors.New("postgres: DSN is required")

	// ErrConnect is returned when the connection pool cannot be created.
	ErrConnect = errors.New("postgres: failed to connect")

	// ErrMigration is returned when the schema migration fails.
	ErrMigration = errors.New("postgres: schema migration failed")
)
