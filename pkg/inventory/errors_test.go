package inventory

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolsetDoesNotExistError(t *testing.T) {
	err := NewToolsetDoesNotExistError("repos")
	assert.Equal(t, "repos", err.Name)
	assert.Equal(t, "toolset repos does not exist", err.Error())

	// Is matches any *ToolsetDoesNotExistError regardless of Name.
	assert.True(t, errors.Is(err, &ToolsetDoesNotExistError{}))
	assert.True(t, errors.Is(err, NewToolsetDoesNotExistError("other")))

	// nil target, unrelated errors, and the sibling error type do not match.
	assert.False(t, err.Is(nil))
	assert.False(t, errors.Is(err, errors.New("boom")))
	assert.False(t, errors.Is(err, NewToolDoesNotExistError("x")))
}

func TestToolDoesNotExistError(t *testing.T) {
	err := NewToolDoesNotExistError("get_me")
	assert.Equal(t, "get_me", err.Name)
	assert.Equal(t, "tool get_me does not exist", err.Error())
}
