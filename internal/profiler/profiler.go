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

// Profile 表示操作的性能指标。
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

// String 返回 profile 的便于阅读的表示形式。
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

// Profiler 提供最小化的性能分析能力。
type Profiler struct {
	logger  *slog.Logger
	enabled bool
}

// New 创建新的 Profiler 实例。
func New(logger *slog.Logger, enabled bool) *Profiler {
	return &Profiler{
		logger:  logger,
		enabled: enabled,
	}
}

// ProfileFunc 分析函数执行的性能。
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

// ProfileFuncWithMetrics 分析函数执行的性能并捕获额外指标。
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

// Start 开始为操作计时，并返回用于完成性能分析的函数。
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

// IsProfilingEnabled 检查是否通过环境变量启用了性能分析。
func IsProfilingEnabled() bool {
	if enabled, err := strconv.ParseBool(os.Getenv("GITHUB_MCP_PROFILING_ENABLED")); err == nil {
		return enabled
	}
	return false
}

// Init 初始化全局 profiler。
func Init(logger *slog.Logger, enabled bool) {
	globalProfiler = New(logger, enabled)
}

// InitFromEnv 使用环境变量初始化全局 profiler。
func InitFromEnv(logger *slog.Logger) {
	globalProfiler = New(logger, IsProfilingEnabled())
}

// ProfileFunc 使用全局 profiler 分析函数性能。
func ProfileFunc(ctx context.Context, operation string, fn func() error) (*Profile, error) {
	if globalProfiler == nil {
		return nil, fn()
	}
	return globalProfiler.ProfileFunc(ctx, operation, fn)
}

// ProfileFuncWithMetrics 使用全局 profiler 分析函数性能并采集指标。
func ProfileFuncWithMetrics(ctx context.Context, operation string, fn func() (int, int64, error)) (*Profile, error) {
	if globalProfiler == nil {
		_, _, err := fn()
		return nil, err
	}
	return globalProfiler.ProfileFuncWithMetrics(ctx, operation, fn)
}

// Start 使用全局 profiler 开始计时。
func Start(ctx context.Context, operation string) func(int, int64) *Profile {
	if globalProfiler == nil {
		return func(int, int64) *Profile { return nil }
	}
	return globalProfiler.Start(ctx, operation)
}
