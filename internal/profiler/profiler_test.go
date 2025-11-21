package profiler

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_safeMemoryDelta_NormalValues(t *testing.T) {
	tests := []struct {
		name     string
		after    uint64
		before   uint64
		expected int64
	}{
		{
			name:     "positive delta",
			after:    1000,
			before:   500,
			expected: 500,
		},
		{
			name:     "negative delta",
			after:    500,
			before:   1000,
			expected: -500,
		},
		{
			name:     "zero delta",
			after:    1000,
			before:   1000,
			expected: 0,
		},
		{
			name:     "large positive delta",
			after:    math.MaxInt64 / 2,
			before:   0,
			expected: math.MaxInt64 / 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := safeMemoryDelta(tc.after, tc.before)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_safeMemoryDelta_OverflowHandling(t *testing.T) {
	tests := []struct {
		name     string
		after    uint64
		before   uint64
		expected int64
	}{
		{
			name:     "after > MaxInt64, positive overflow",
			after:    math.MaxUint64,
			before:   0,
			expected: math.MaxInt64, // Capped at MaxInt64
		},
		{
			name:     "before > MaxInt64, negative overflow",
			after:    0,
			before:   math.MaxUint64,
			expected: -math.MaxInt64, // Capped at -MaxInt64
		},
		{
			name:     "both > MaxInt64, positive delta",
			after:    math.MaxUint64,
			before:   math.MaxUint64 - 1000,
			expected: 1000,
		},
		{
			name:     "both > MaxInt64, negative delta",
			after:    math.MaxUint64 - 1000,
			before:   math.MaxUint64,
			expected: -1000,
		},
		{
			name:     "both > MaxInt64, equal",
			after:    math.MaxUint64,
			before:   math.MaxUint64,
			expected: 0,
		},
		{
			name:     "after at MaxInt64 boundary",
			after:    uint64(math.MaxInt64),
			before:   0,
			expected: math.MaxInt64,
		},
		{
			name:     "delta exceeds MaxInt64",
			after:    math.MaxUint64,
			before:   math.MaxUint64 - uint64(math.MaxInt64) - 1000,
			expected: math.MaxInt64, // Overflow, capped
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := safeMemoryDelta(tc.after, tc.before)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_Profile_String(t *testing.T) {
	profile := &Profile{
		Operation:   "test_operation",
		MemoryDelta: 1024,
		LinesCount:  10,
		BytesCount:  5000,
	}

	str := profile.String()

	assert.Contains(t, str, "test_operation")
	assert.Contains(t, str, "1024B")
	assert.Contains(t, str, "lines=10")
	assert.Contains(t, str, "bytes=5000")
}

func Test_New_Profiler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name    string
		logger  *slog.Logger
		enabled bool
	}{
		{
			name:    "enabled with logger",
			logger:  logger,
			enabled: true,
		},
		{
			name:    "disabled with logger",
			logger:  logger,
			enabled: false,
		},
		{
			name:    "enabled without logger",
			logger:  nil,
			enabled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := New(tc.logger, tc.enabled)
			require.NotNil(t, p)
			assert.Equal(t, tc.enabled, p.enabled)
			assert.Equal(t, tc.logger, p.logger)
		})
	}
}

func Test_ProfileFunc_Disabled(t *testing.T) {
	p := New(nil, false)
	ctx := context.Background()

	executed := false
	fn := func() error {
		executed = true
		return nil
	}

	profile, err := p.ProfileFunc(ctx, "test", fn)

	require.NoError(t, err)
	assert.Nil(t, profile, "Profile should be nil when disabled")
	assert.True(t, executed, "Function should still execute when profiling disabled")
}

func Test_ProfileFunc_Enabled(t *testing.T) {
	p := New(nil, true)
	ctx := context.Background()

	executed := false
	fn := func() error {
		executed = true
		return nil
	}

	profile, err := p.ProfileFunc(ctx, "test_operation", fn)

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.True(t, executed)
	assert.Equal(t, "test_operation", profile.Operation)
	assert.NotZero(t, profile.Duration)
	assert.NotZero(t, profile.Timestamp)
}

func Test_ProfileFunc_ReturnsError(t *testing.T) {
	p := New(nil, true)
	ctx := context.Background()

	expectedErr := errors.New("test error")
	fn := func() error {
		return expectedErr
	}

	profile, err := p.ProfileFunc(ctx, "test", fn)

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	require.NotNil(t, profile, "Profile should still be captured on error")
}

func Test_ProfileFuncWithMetrics_Disabled(t *testing.T) {
	p := New(nil, false)
	ctx := context.Background()

	fn := func() (int, int64, error) {
		return 100, 5000, nil
	}

	profile, err := p.ProfileFuncWithMetrics(ctx, "test", fn)

	require.NoError(t, err)
	assert.Nil(t, profile, "Profile should be nil when disabled")
}

func Test_ProfileFuncWithMetrics_Enabled(t *testing.T) {
	p := New(nil, true)
	ctx := context.Background()

	fn := func() (int, int64, error) {
		return 100, 5000, nil
	}

	profile, err := p.ProfileFuncWithMetrics(ctx, "test_metrics", fn)

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "test_metrics", profile.Operation)
	assert.Equal(t, 100, profile.LinesCount)
	assert.Equal(t, int64(5000), profile.BytesCount)
	assert.NotZero(t, profile.Duration)
}

func Test_ProfileFuncWithMetrics_ReturnsError(t *testing.T) {
	p := New(nil, true)
	ctx := context.Background()

	expectedErr := errors.New("metrics error")
	fn := func() (int, int64, error) {
		return 50, 1000, expectedErr
	}

	profile, err := p.ProfileFuncWithMetrics(ctx, "test", fn)

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	require.NotNil(t, profile)
	assert.Equal(t, 50, profile.LinesCount)
	assert.Equal(t, int64(1000), profile.BytesCount)
}

func Test_Start_Disabled(t *testing.T) {
	p := New(nil, false)
	ctx := context.Background()

	finish := p.Start(ctx, "test")
	require.NotNil(t, finish)

	profile := finish(10, 500)
	assert.Nil(t, profile, "Profile should be nil when disabled")
}

func Test_Start_Enabled(t *testing.T) {
	p := New(nil, true)
	ctx := context.Background()

	finish := p.Start(ctx, "test_start")
	require.NotNil(t, finish)

	profile := finish(25, 1200)

	require.NotNil(t, profile)
	assert.Equal(t, "test_start", profile.Operation)
	assert.Equal(t, 25, profile.LinesCount)
	assert.Equal(t, int64(1200), profile.BytesCount)
	assert.NotZero(t, profile.Duration)
}

func Test_IsProfilingEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "enabled with true",
			envValue: "true",
			expected: true,
		},
		{
			name:     "enabled with 1",
			envValue: "1",
			expected: true,
		},
		{
			name:     "disabled with false",
			envValue: "false",
			expected: false,
		},
		{
			name:     "disabled with 0",
			envValue: "0",
			expected: false,
		},
		{
			name:     "disabled with empty",
			envValue: "",
			expected: false,
		},
		{
			name:     "disabled with invalid value",
			envValue: "invalid",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Save and restore original env var
			originalValue := os.Getenv("GITHUB_MCP_PROFILING_ENABLED")
			defer func() {
				if originalValue != "" {
					os.Setenv("GITHUB_MCP_PROFILING_ENABLED", originalValue)
				} else {
					os.Unsetenv("GITHUB_MCP_PROFILING_ENABLED")
				}
			}()

			if tc.envValue != "" {
				os.Setenv("GITHUB_MCP_PROFILING_ENABLED", tc.envValue)
			} else {
				os.Unsetenv("GITHUB_MCP_PROFILING_ENABLED")
			}

			result := IsProfilingEnabled()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_Init_GlobalProfiler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	Init(logger, true)
	assert.NotNil(t, globalProfiler)
	assert.True(t, globalProfiler.enabled)

	Init(logger, false)
	assert.NotNil(t, globalProfiler)
	assert.False(t, globalProfiler.enabled)
}

func Test_InitFromEnv_GlobalProfiler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Save and restore original env var
	originalValue := os.Getenv("GITHUB_MCP_PROFILING_ENABLED")
	defer func() {
		if originalValue != "" {
			os.Setenv("GITHUB_MCP_PROFILING_ENABLED", originalValue)
		} else {
			os.Unsetenv("GITHUB_MCP_PROFILING_ENABLED")
		}
	}()

	os.Setenv("GITHUB_MCP_PROFILING_ENABLED", "true")
	InitFromEnv(logger)
	assert.NotNil(t, globalProfiler)
	assert.True(t, globalProfiler.enabled)

	os.Setenv("GITHUB_MCP_PROFILING_ENABLED", "false")
	InitFromEnv(logger)
	assert.NotNil(t, globalProfiler)
	assert.False(t, globalProfiler.enabled)
}

func Test_GlobalProfileFunc_NilGlobalProfiler(t *testing.T) {
	// Save and restore global profiler
	originalProfiler := globalProfiler
	defer func() {
		globalProfiler = originalProfiler
	}()

	globalProfiler = nil
	ctx := context.Background()

	executed := false
	fn := func() error {
		executed = true
		return nil
	}

	profile, err := ProfileFunc(ctx, "test", fn)

	require.NoError(t, err)
	assert.Nil(t, profile)
	assert.True(t, executed)
}

func Test_GlobalProfileFunc_WithGlobalProfiler(t *testing.T) {
	// Save and restore global profiler
	originalProfiler := globalProfiler
	defer func() {
		globalProfiler = originalProfiler
	}()

	Init(nil, true)
	ctx := context.Background()

	executed := false
	fn := func() error {
		executed = true
		return nil
	}

	profile, err := ProfileFunc(ctx, "global_test", fn)

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.True(t, executed)
	assert.Equal(t, "global_test", profile.Operation)
}

func Test_GlobalProfileFuncWithMetrics_NilGlobalProfiler(t *testing.T) {
	// Save and restore global profiler
	originalProfiler := globalProfiler
	defer func() {
		globalProfiler = originalProfiler
	}()

	globalProfiler = nil
	ctx := context.Background()

	fn := func() (int, int64, error) {
		return 10, 100, nil
	}

	profile, err := ProfileFuncWithMetrics(ctx, "test", fn)

	require.NoError(t, err)
	assert.Nil(t, profile)
}

func Test_GlobalProfileFuncWithMetrics_WithGlobalProfiler(t *testing.T) {
	// Save and restore global profiler
	originalProfiler := globalProfiler
	defer func() {
		globalProfiler = originalProfiler
	}()

	Init(nil, true)
	ctx := context.Background()

	fn := func() (int, int64, error) {
		return 42, 9999, nil
	}

	profile, err := ProfileFuncWithMetrics(ctx, "global_metrics", fn)

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.Equal(t, "global_metrics", profile.Operation)
	assert.Equal(t, 42, profile.LinesCount)
	assert.Equal(t, int64(9999), profile.BytesCount)
}

func Test_GlobalStart_NilGlobalProfiler(t *testing.T) {
	// Save and restore global profiler
	originalProfiler := globalProfiler
	defer func() {
		globalProfiler = originalProfiler
	}()

	globalProfiler = nil
	ctx := context.Background()

	finish := Start(ctx, "test")
	require.NotNil(t, finish)

	profile := finish(5, 50)
	assert.Nil(t, profile)
}

func Test_GlobalStart_WithGlobalProfiler(t *testing.T) {
	// Save and restore global profiler
	originalProfiler := globalProfiler
	defer func() {
		globalProfiler = originalProfiler
	}()

	Init(nil, true)
	ctx := context.Background()

	finish := Start(ctx, "global_start")
	require.NotNil(t, finish)

	profile := finish(15, 750)

	require.NotNil(t, profile)
	assert.Equal(t, "global_start", profile.Operation)
	assert.Equal(t, 15, profile.LinesCount)
	assert.Equal(t, int64(750), profile.BytesCount)
}
