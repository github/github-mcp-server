package profiler

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"

	"log/slog"
	"math"
)

// Profile represents performance metrics for an operation
type Profile struct {
	Operation    string        `json:"operation"`
	Duration     time.Duration `json:"duration_ns"`
	MemoryBefore uint64        `json:"memory_before_bytes"`
	MemoryAfter  uint64        `json:"memory_after_bytes"`
	MemoryDelta  int64         `json:"memory_delta_bytes"`
	LinesCount   int           `json:"lines_count,omitempty"`
	BytesCount   int64         `json:"bytes_count,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
}

// String returns a human-readable representation of the profile
func (p *Profile) String() string {
	return fmt.Sprintf("[%s] %s: duration=%v, memory_delta=%+dB, lines=%d, bytes=%d",
		p.Timestamp.Format("15:04:05.000"),
		p.Operation,
		p.Duration,
		p.MemoryDelta,
		p.LinesCount,
		p.BytesCount,
	)
}

func safeMemoryDelta(after, before uint64) int64 {
	if after > math.MaxInt64 || before > math.MaxInt64 {
		if after >= before {
			diff := after - before
			if diff > math.MaxInt64 {
				return math.MaxInt64
			}
			return int64(diff)
		}
		diff := before - after
		if diff > math.MaxInt64 {
			return -math.MaxInt64
		}
		return -int64(diff)
	}

	return int64(after) - int64(before)
}

// Profiler provides minimal performance profiling capabilities
type Profiler struct {
	logger  *slog.Logger
	enabled bool
}

// New creates a new Profiler instance
func New(logger *slog.Logger, enabled bool) *Profiler {
	return &Profiler{
		logger:  logger,
		enabled: enabled,
	}
}

// ProfileFunc profiles a function execution
func (p *Profiler) ProfileFunc(ctx context.Context, operation string, fn func() error) (*Profile, error) {
	if !p.enabled {
		return nil, fn()
	}

	profile := &Profile{
		Operation: operation,
		Timestamp: time.Now(),
	}

	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	profile.MemoryBefore = memBefore.Alloc

	start := time.Now()
	err := fn()
	profile.Duration = time.Since(start)

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	profile.MemoryAfter = memAfter.Alloc
	profile.MemoryDelta = safeMemoryDelta(memAfter.Alloc, memBefore.Alloc)

	if p.logger != nil {
		p.logger.InfoContext(ctx, "Performance profile", "profile", profile.String())
	}

	return profile, err
}

// ProfileFuncWithMetrics profiles a function execution and captures additional metrics
func (p *Profiler) ProfileFuncWithMetrics(ctx context.Context, operation string, fn func() (int, int64, error)) (*Profile, error) {
	if !p.enabled {
		_, _, err := fn()
		return nil, err
	}

	profile := &Profile{
		Operation: operation,
		Timestamp: time.Now(),
	}

	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	profile.MemoryBefore = memBefore.Alloc

	start := time.Now()
	lines, bytes, err := fn()
	profile.Duration = time.Since(start)
	profile.LinesCount = lines
	profile.BytesCount = bytes

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	profile.MemoryAfter = memAfter.Alloc
	profile.MemoryDelta = safeMemoryDelta(memAfter.Alloc, memBefore.Alloc)

	if p.logger != nil {
		p.logger.InfoContext(ctx, "Performance profile", "profile", profile.String())
	}

	return profile, err
}

// Start begins timing an operation and returns a function to complete the profiling
func (p *Profiler) Start(ctx context.Context, operation string) func(lines int, bytes int64) *Profile {
	if !p.enabled {
		return func(int, int64) *Profile { return nil }
	}

	profile := &Profile{
		Operation: operation,
		Timestamp: time.Now(),
	}

	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	profile.MemoryBefore = memBefore.Alloc

	start := time.Now()

	return func(lines int, bytes int64) *Profile {
		profile.Duration = time.Since(start)
		profile.LinesCount = lines
		profile.BytesCount = bytes

		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)
		profile.MemoryAfter = memAfter.Alloc
		profile.MemoryDelta = safeMemoryDelta(memAfter.Alloc, memBefore.Alloc)

		if p.logger != nil {
			p.logger.InfoContext(ctx, "Performance profile", "profile", profile.String())
		}

		return profile
	}
}

var globalProfiler *Profiler

// IsProfilingEnabled checks if profiling is enabled via environment variables
func IsProfilingEnabled() bool {
	if enabled, err := strconv.ParseBool(os.Getenv("GITHUB_MCP_PROFILING_ENABLED")); err == nil {
		return enabled
	}
	return false
}

// Init initializes the global profiler
func Init(logger *slog.Logger, enabled bool) {
	globalProfiler = New(logger, enabled)
}

// InitFromEnv initializes the global profiler using environment variables
func InitFromEnv(logger *slog.Logger) {
	globalProfiler = New(logger, IsProfilingEnabled())
}

// ProfileFunc profiles a function using the global profiler
func ProfileFunc(ctx context.Context, operation string, fn func() error) (*Profile, error) {
	if globalProfiler == nil {
		return nil, fn()
	}
	return globalProfiler.ProfileFunc(ctx, operation, fn)
}

// ProfileFuncWithMetrics profiles a function with metrics using the global profiler
func ProfileFuncWithMetrics(ctx context.Context, operation string, fn func() (int, int64, error)) (*Profile, error) {
	if globalProfiler == nil {
		_, _, err := fn()
		return nil, err
	}
	return globalProfiler.ProfileFuncWithMetrics(ctx, operation, fn)
}

// Start begins timing using the global profiler
func Start(ctx context.Context, operation string) func(int, int64) *Profile {
	if globalProfiler == nil {
		return func(int, int64) *Profile { return nil }
	}
	return globalProfiler.Start(ctx, operation)
}
