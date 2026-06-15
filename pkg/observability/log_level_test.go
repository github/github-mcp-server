package observability

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLogLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{" info ", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
	}
	for _, tc := range cases {
		got, err := ParseLogLevel(tc.in, slog.LevelInfo)
		require.NoError(t, err, tc.in)
		assert.Equal(t, tc.want, got, tc.in)
	}
}

func TestParseLogLevel_EmptyReturnsDefault(t *testing.T) {
	got, err := ParseLogLevel("", slog.LevelWarn)
	require.NoError(t, err)
	assert.Equal(t, slog.LevelWarn, got)
}

func TestParseLogLevel_Unknown(t *testing.T) {
	got, err := ParseLogLevel("verbose", slog.LevelInfo)
	require.Error(t, err)
	assert.Equal(t, slog.LevelInfo, got)
}
