package inmemory

import "time"

type config struct {
	ttl         time.Duration // 0 = no TTL
	maxMessages int           // 0 = unlimited
}

// Option configures a Store.
type Option func(*config)

// WithTTL sets a session expiry duration. Sessions that have not been accessed
// (via Get or Append) within d are cleared on the next Get call.
func WithTTL(d time.Duration) Option {
	return func(c *config) { c.ttl = d }
}

// WithMaxMessages limits the number of messages stored per session.
// When the limit is exceeded the oldest messages are dropped so only the most
// recent n messages are kept. 0 means unlimited (default).
func WithMaxMessages(n int) Option {
	return func(c *config) { c.maxMessages = n }
}
