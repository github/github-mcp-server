package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildInfoStruct_String(t *testing.T) {
	tests := []struct {
		name     string
		info     buildInfoStruct
		expected string
	}{
		{
			name: "all fields populated",
			info: buildInfoStruct{
				commit:  "abc123",
				date:    "2024-01-01",
				version: "1.0.0",
			},
			expected: "Commit: abc123\nBuild Date: 2024-01-01\nVersion: 1.0.0",
		},
		{
			name:     "initialized struct",
			info:     buildInfoStruct{},
			expected: "Commit: \nBuild Date: \nVersion: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildInfo_Initialization(t *testing.T) {
	assert.Equal(t, "commit", buildInfo.commit)
	assert.Equal(t, "date", buildInfo.date)
	assert.Equal(t, "version", buildInfo.version)
}
