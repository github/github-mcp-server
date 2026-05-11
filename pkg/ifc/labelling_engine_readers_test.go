package ifc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabelGetMe(t *testing.T) {
	label := LabelGetMe()
	assert.True(t, label.IsHighIntegrity())
	assert.True(t, label.IsPublicConfidentiality())
}
