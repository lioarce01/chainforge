package redis

import "time"

const (
	defaultAddr      = "localhost:6379"
	defaultKeyPrefix = "chainforge"
	defaultDB        = 0
)

// Config holds all settings for the Redis memory store.
type Config struct {
	Addr      string
	Password  string
	DB        int
	KeyPrefix string
	TTL       time.Duration
}

// Option mutates a Config.
type Option func(*Config)

func defaultConfig() Config {
	return Config{
		Addr:      defaultAddr,
		KeyPrefix: defaultKeyPrefix,
		DB:        defaultDB,
	}
}

// WithAddr sets the Redis server address (host:port).
func WithAddr(addr string) Option {
	return func(c *Config) { c.Addr = addr }
}

// WithPassword sets the Redis AUTH password.
func WithPassword(password string) Option {
	return func(c *Config) { c.Password = password }
}

// WithDB selects the Redis logical database index.
func WithDB(db int) Option {
	return func(c *Config) { c.DB = db }
}

// WithKeyPrefix sets the prefix used for all session keys.
func WithKeyPrefix(prefix string) Option {
	return func(c *Config) { c.KeyPrefix = prefix }
}

// WithTTL sets the sliding-window TTL applied on every Append.
// A zero value means keys never expire (default).
func WithTTL(d time.Duration) Option {
	return func(c *Config) { c.TTL = d }
}
