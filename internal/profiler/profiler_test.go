package profiler

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name    string
		logger  *slog.Logger
		enabled bool
	}{
		{
			name:    "enabled profiler",
			logger:  logger,
			enabled: true,
		},
		{
			name:    "disabled profiler",
			logger:  logger,
			enabled: false,
		},
		{
			name:    "nil logger",
			logger:  nil,
			enabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.logger, tt.enabled)
			assert.NotNil(t, p)
			assert.Equal(t, tt.enabled, p.enabled)
			assert.Equal(t, tt.logger, p.logger)
		})
	}
}

func TestProfileFunc_Enabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := New(logger, true)

	called := false
	fn := func() error {
		called = true
		time.Sleep(10 * time.Millisecond)
		return nil
	}

	profile, err := p.ProfileFunc(context.Background(), "test_operation", fn)

	require.NoError(t, err)
	assert.True(t, called)
	assert.NotNil(t, profile)
	assert.Equal(t, "test_operation", profile.Operation)
	assert.Greater(t, profile.Duration, time.Duration(0))
	assert.NotZero(t, profile.Timestamp)
}

func TestProfileFunc_Disabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := New(logger, false)

	called := false
	fn := func() error {
		called = true
		return nil
	}

	profile, err := p.ProfileFunc(context.Background(), "test_operation", fn)

	require.NoError(t, err)
	assert.True(t, called)
	assert.Nil(t, profile, "Profile should be nil when disabled")
}

func TestProfileFunc_FunctionError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := New(logger, true)

	expectedErr := errors.New("test error")
	fn := func() error {
		return expectedErr
	}

	profile, err := p.ProfileFunc(context.Background(), "test_operation", fn)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.NotNil(t, profile, "Profile should still be returned even on error")
	assert.Equal(t, "test_operation", profile.Operation)
}

func TestProfileFuncWithMetrics_Enabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := New(logger, true)

	fn := func() (int, int64, error) {
		return 42, 1024, nil
	}

	profile, err := p.ProfileFuncWithMetrics(context.Background(), "test_operation", fn)

	require.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, "test_operation", profile.Operation)
	assert.Equal(t, 42, profile.LinesCount)
	assert.Equal(t, int64(1024), profile.BytesCount)
	assert.Greater(t, profile.Duration, time.Duration(0))
}

func TestProfileFuncWithMetrics_Disabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := New(logger, false)

	called := false
	fn := func() (int, int64, error) {
		called = true
		return 10, 100, nil
	}

	profile, err := p.ProfileFuncWithMetrics(context.Background(), "test_operation", fn)

	require.NoError(t, err)
	assert.True(t, called)
	assert.Nil(t, profile, "Profile should be nil when disabled")
}

func TestProfileFuncWithMetrics_Error(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := New(logger, true)

	expectedErr := errors.New("test error")
	fn := func() (int, int64, error) {
		return 5, 50, expectedErr
	}

	profile, err := p.ProfileFuncWithMetrics(context.Background(), "test_operation", fn)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.NotNil(t, profile)
	assert.Equal(t, 5, profile.LinesCount)
	assert.Equal(t, int64(50), profile.BytesCount)
}

func TestStart_Enabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := New(logger, true)

	done := p.Start(context.Background(), "test_operation")
	time.Sleep(10 * time.Millisecond)
	profile := done(100, 2048)

	assert.NotNil(t, profile)
	assert.Equal(t, "test_operation", profile.Operation)
	assert.Equal(t, 100, profile.LinesCount)
	assert.Equal(t, int64(2048), profile.BytesCount)
	assert.Greater(t, profile.Duration, 10*time.Millisecond)
}

func TestStart_Disabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := New(logger, false)

	done := p.Start(context.Background(), "test_operation")
	profile := done(100, 2048)

	assert.Nil(t, profile, "Profile should be nil when disabled")
}

func TestProfileString(t *testing.T) {
	profile := &Profile{
		Operation:    "test_op",
		Duration:     100 * time.Millisecond,
		MemoryBefore: 1000,
		MemoryAfter:  2000,
		MemoryDelta:  1000,
		LinesCount:   42,
		BytesCount:   1024,
		Timestamp:    time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC),
	}

	str := profile.String()
	assert.Contains(t, str, "test_op")
	assert.Contains(t, str, "100ms")
	assert.Contains(t, str, "42")
	assert.Contains(t, str, "1024")
}

func TestSafeMemoryDelta(t *testing.T) {
	tests := []struct {
		name   string
		after  uint64
		before uint64
		want   int64
	}{
		{
			name:   "positive delta",
			after:  2000,
			before: 1000,
			want:   1000,
		},
		{
			name:   "negative delta",
			after:  1000,
			before: 2000,
			want:   -1000,
		},
		{
			name:   "zero delta",
			after:  1000,
			before: 1000,
			want:   0,
		},
		{
			name:   "large positive delta",
			after:  math.MaxInt64,
			before: 0,
			want:   math.MaxInt64,
		},
		{
			name:   "overflow positive",
			after:  math.MaxUint64,
			before: 0,
			want:   math.MaxInt64,
		},
		{
			name:   "overflow negative",
			after:  0,
			before: math.MaxUint64,
			want:   -math.MaxInt64,
		},
		{
			name:   "both very large, positive delta",
			after:  math.MaxUint64,
			before: math.MaxUint64 - 100,
			want:   100,
		},
		{
			name:   "both very large, negative delta",
			after:  math.MaxUint64 - 100,
			before: math.MaxUint64,
			want:   -100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeMemoryDelta(tt.after, tt.before)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsProfilingEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "true",
			envValue: "true",
			want:     true,
		},
		{
			name:     "false",
			envValue: "false",
			want:     false,
		},
		{
			name:     "1",
			envValue: "1",
			want:     true,
		},
		{
			name:     "0",
			envValue: "0",
			want:     false,
		},
		{
			name:     "empty",
			envValue: "",
			want:     false,
		},
		{
			name:     "invalid",
			envValue: "invalid",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" {
				os.Unsetenv("GITHUB_MCP_PROFILING_ENABLED")
			} else {
				os.Setenv("GITHUB_MCP_PROFILING_ENABLED", tt.envValue)
			}
			defer os.Unsetenv("GITHUB_MCP_PROFILING_ENABLED")

			got := IsProfilingEnabled()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	Init(logger, true)
	assert.NotNil(t, globalProfiler)
	assert.True(t, globalProfiler.enabled)

	Init(logger, false)
	assert.NotNil(t, globalProfiler)
	assert.False(t, globalProfiler.enabled)
}

func TestInitFromEnv(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	os.Setenv("GITHUB_MCP_PROFILING_ENABLED", "true")
	defer os.Unsetenv("GITHUB_MCP_PROFILING_ENABLED")

	InitFromEnv(logger)
	assert.NotNil(t, globalProfiler)
	assert.True(t, globalProfiler.enabled)
}

func TestGlobalProfileFunc(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	Init(logger, true)

	called := false
	fn := func() error {
		called = true
		return nil
	}

	profile, err := ProfileFunc(context.Background(), "test_operation", fn)

	require.NoError(t, err)
	assert.True(t, called)
	assert.NotNil(t, profile)
}

func TestGlobalProfileFunc_NilProfiler(t *testing.T) {
	// Set global profiler to nil
	globalProfiler = nil

	called := false
	fn := func() error {
		called = true
		return nil
	}

	profile, err := ProfileFunc(context.Background(), "test_operation", fn)

	require.NoError(t, err)
	assert.True(t, called)
	assert.Nil(t, profile)
}

func TestGlobalProfileFuncWithMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	Init(logger, true)

	fn := func() (int, int64, error) {
		return 10, 100, nil
	}

	profile, err := ProfileFuncWithMetrics(context.Background(), "test_operation", fn)

	require.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Equal(t, 10, profile.LinesCount)
	assert.Equal(t, int64(100), profile.BytesCount)
}

func TestGlobalProfileFuncWithMetrics_NilProfiler(t *testing.T) {
	// Set global profiler to nil
	globalProfiler = nil

	called := false
	fn := func() (int, int64, error) {
		called = true
		return 5, 50, nil
	}

	profile, err := ProfileFuncWithMetrics(context.Background(), "test_operation", fn)

	require.NoError(t, err)
	assert.True(t, called)
	assert.Nil(t, profile)
}

func TestGlobalStart(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	Init(logger, true)

	done := Start(context.Background(), "test_operation")
	time.Sleep(5 * time.Millisecond)
	profile := done(5, 50)

	assert.NotNil(t, profile)
	assert.Equal(t, "test_operation", profile.Operation)
	assert.Equal(t, 5, profile.LinesCount)
	assert.Equal(t, int64(50), profile.BytesCount)
}

func TestGlobalStart_NilProfiler(t *testing.T) {
	// Set global profiler to nil
	globalProfiler = nil

	done := Start(context.Background(), "test_operation")
	profile := done(5, 50)

	assert.Nil(t, profile)
}

func TestProfile_AllFieldsPopulated(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := New(logger, true)

	fn := func() (int, int64, error) {
		// Allocate some memory
		_ = make([]byte, 1024)
		time.Sleep(5 * time.Millisecond)
		return 42, 2048, nil
	}

	profile, err := p.ProfileFuncWithMetrics(context.Background(), "test_operation", fn)

	require.NoError(t, err)
	assert.NotNil(t, profile)

	// Verify all fields are populated
	assert.Equal(t, "test_operation", profile.Operation)
	assert.Greater(t, profile.Duration, time.Duration(0))
	assert.NotZero(t, profile.MemoryBefore)
	assert.NotZero(t, profile.MemoryAfter)
	// Memory delta could be positive or negative due to GC
	assert.NotZero(t, profile.MemoryDelta)
	assert.Equal(t, 42, profile.LinesCount)
	assert.Equal(t, int64(2048), profile.BytesCount)
	assert.False(t, profile.Timestamp.IsZero())
}

func TestProfileFunc_NilLogger(t *testing.T) {
	// Test that profiler works with nil logger
	p := New(nil, true)

	fn := func() error {
		return nil
	}

	profile, err := p.ProfileFunc(context.Background(), "test_operation", fn)

	require.NoError(t, err)
	assert.NotNil(t, profile)
}

func TestProfileFunc_Duration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := New(logger, true)

	sleepDuration := 50 * time.Millisecond
	fn := func() error {
		time.Sleep(sleepDuration)
		return nil
	}

	profile, err := p.ProfileFunc(context.Background(), "test_operation", fn)

	require.NoError(t, err)
	assert.NotNil(t, profile)
	// Duration should be at least as long as the sleep
	assert.GreaterOrEqual(t, profile.Duration, sleepDuration)
}

func TestMemoryDelta_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		after  uint64
		before uint64
	}{
		{
			name:   "max uint64 after",
			after:  math.MaxUint64,
			before: 1000,
		},
		{
			name:   "max uint64 before",
			after:  1000,
			before: math.MaxUint64,
		},
		{
			name:   "both max uint64",
			after:  math.MaxUint64,
			before: math.MaxUint64,
		},
		{
			name:   "both zero",
			after:  0,
			before: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			delta := safeMemoryDelta(tt.after, tt.before)
			// Delta should be within int64 range
			assert.LessOrEqual(t, delta, int64(math.MaxInt64))
			assert.GreaterOrEqual(t, delta, -int64(math.MaxInt64))
		})
	}
}

func TestStart_MultipleInvocations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	p := New(logger, true)

	// Start multiple profiling sessions
	done1 := p.Start(context.Background(), "operation1")
	time.Sleep(5 * time.Millisecond)
	done2 := p.Start(context.Background(), "operation2")
	time.Sleep(5 * time.Millisecond)

	profile1 := done1(10, 100)
	profile2 := done2(20, 200)

	assert.NotNil(t, profile1)
	assert.NotNil(t, profile2)
	assert.Equal(t, "operation1", profile1.Operation)
	assert.Equal(t, "operation2", profile2.Operation)
	assert.Greater(t, profile1.Duration, profile2.Duration)
}

func TestProfileString_Formatting(t *testing.T) {
	profile := &Profile{
		Operation:    "format_test",
		Duration:     123456789 * time.Nanosecond,
		MemoryDelta:  -500,
		LinesCount:   0,
		BytesCount:   0,
		Timestamp:    time.Date(2024, 6, 15, 14, 30, 45, 123456789, time.UTC),
	}

	str := profile.String()

	// Verify format
	assert.Contains(t, str, "format_test")
	assert.Contains(t, str, "duration")
	assert.Contains(t, str, "memory_delta")
	// Negative memory delta should be represented
	assert.Contains(t, str, "-500")
}
