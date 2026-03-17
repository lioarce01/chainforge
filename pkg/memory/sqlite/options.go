package sqlite

import "time"

const (
	defaultPath        = "chainforge.db"
	defaultTableName   = "chainforge_messages"
	defaultBusyTimeout = 5 * time.Second
)

// Config holds all settings for the SQLite memory store.
type Config struct {
	Path        string
	TableName   string
	BusyTimeout time.Duration
}

// Option mutates a Config.
type Option func(*Config)

func defaultConfig() Config {
	return Config{
		Path:        defaultPath,
		TableName:   defaultTableName,
		BusyTimeout: defaultBusyTimeout,
	}
}

// WithPath sets the SQLite database file path.
func WithPath(path string) Option {
	return func(c *Config) { c.Path = path }
}

// WithTableName overrides the default table name.
func WithTableName(name string) Option {
	return func(c *Config) { c.TableName = name }
}

// WithBusyTimeout sets how long SQLite waits when the database is locked.
func WithBusyTimeout(d time.Duration) Option {
	return func(c *Config) { c.BusyTimeout = d }
}
