package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_RootCmdVersion(t *testing.T) {
	expectedVersion := buildInfo.String()
	actualVersion := rootCmd.Version

	assert.Equal(t, expectedVersion, actualVersion)
}
