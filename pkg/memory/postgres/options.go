package postgres

const (
	defaultTableName  = "chainforge_messages"
	defaultSchemaName = "public"
	defaultMaxConns   = int32(10)
)

// Config holds all settings for the PostgreSQL memory store.
type Config struct {
	DSN        string
	TableName  string
	SchemaName string
	MaxConns   int32
}

// Option mutates a Config.
type Option func(*Config)

func defaultConfig() Config {
	return Config{
		TableName:  defaultTableName,
		SchemaName: defaultSchemaName,
		MaxConns:   defaultMaxConns,
	}
}

// WithDSN sets the PostgreSQL connection string (required).
func WithDSN(dsn string) Option {
	return func(c *Config) { c.DSN = dsn }
}

// WithTableName overrides the default table name.
func WithTableName(name string) Option {
	return func(c *Config) { c.TableName = name }
}

// WithSchemaName overrides the default schema name.
func WithSchemaName(name string) Option {
	return func(c *Config) { c.SchemaName = name }
}

// WithMaxConns sets the maximum number of connections in the pool.
func WithMaxConns(n int32) Option {
	return func(c *Config) { c.MaxConns = n }
}
