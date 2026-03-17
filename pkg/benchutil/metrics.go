package benchutil

import (
	"fmt"
	"io"
	"sort"
	"time"
)

// LatencyRecorder accumulates request durations for percentile analysis.
type LatencyRecorder struct {
	samples []time.Duration
}

// Record adds a single latency measurement.
func (r *LatencyRecorder) Record(d time.Duration) {
	r.samples = append(r.samples, d)
}

// Len returns the number of recorded samples.
func (r *LatencyRecorder) Len() int { return len(r.samples) }

// Percentile returns the p-th percentile latency (0–100).
// Returns 0 if no samples have been recorded.
func (r *LatencyRecorder) Percentile(p float64) time.Duration {
	if len(r.samples) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(r.samples))
	copy(sorted, r.samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	idx := int(float64(len(sorted)-1) * p / 100.0)
	return sorted[idx]
}

// Mean returns the arithmetic mean latency.
func (r *LatencyRecorder) Mean() time.Duration {
	if len(r.samples) == 0 {
		return 0
	}
	var sum time.Duration
	for _, s := range r.samples {
		sum += s
	}
	return sum / time.Duration(len(r.samples))
}

// Summary holds a human-readable latency summary.
type Summary struct {
	N    int
	P50  time.Duration
	P95  time.Duration
	P99  time.Duration
	Mean time.Duration
}

// Summarize computes the full percentile summary.
func (r *LatencyRecorder) Summarize() Summary {
	return Summary{
		N:    r.Len(),
		P50:  r.Percentile(50),
		P95:  r.Percentile(95),
		P99:  r.Percentile(99),
		Mean: r.Mean(),
	}
}

// Print writes the summary in a table format.
func (s Summary) Print(w io.Writer) {
	fmt.Fprintf(w, "  requests : %d\n", s.N)
	fmt.Fprintf(w, "  p50      : %s\n", s.P50)
	fmt.Fprintf(w, "  p95      : %s\n", s.P95)
	fmt.Fprintf(w, "  p99      : %s\n", s.P99)
	fmt.Fprintf(w, "  mean     : %s\n", s.Mean)
}

// ThroughputRPS calculates requests-per-second given total duration.
func ThroughputRPS(n int, total time.Duration) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / total.Seconds()
}

// TokensPerSecond calculates token throughput.
func TokensPerSecond(tokens int, d time.Duration) float64 {
	if d == 0 {
		return 0
	}
	return float64(tokens) / d.Seconds()
}
