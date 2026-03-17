package redis

import "errors"

var (
	// ErrNoAddr is returned by New when no address is provided.
	ErrNoAddr = errors.New("redis: address is required")

	// ErrConnect is returned when the initial Ping fails.
	ErrConnect = errors.New("redis: failed to connect")
)
