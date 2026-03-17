// Package middleware provides opt-in observability wrappers for chainforge
// core interfaces. Wrappers are composable: wrap a Provider in logging, then
// wrap that in OTel tracing, without modifying any core package.
//
// # Logging
//
// Use pkg/middleware/logging for zero-dependency structured logging via
// stdlib log/slog. Useful in development and production alike.
//
// # OTel Tracing
//
// Use pkg/middleware/otel for OpenTelemetry distributed tracing. Requires
// an OTLP-compatible backend (Jaeger, Tempo, etc.). Import only when you
// need tracing — this keeps the core packages dependency-free.
//
// # Usage
//
//	// At startup — no changes to agent or core packages required.
//	raw, _ := providers.NewFromConfig(cfg)
//	logged  := logging.NewLoggedProvider(raw, slog.Default())
//	traced  := otel.NewTracedProvider(logged, tracer)
//
//	agent, _ := chainforge.NewAgent(
//	    chainforge.WithProvider(traced),
//	    chainforge.WithMemory(otel.NewTracedMemoryStore(mem, tracer)),
//	)
package middleware
