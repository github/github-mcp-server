package inventory

import "fmt"

// ToolsetDoesNotExistError is returned when a toolset is not found.
type ToolsetDoesNotExistError struct {
	Name string
}

func (e *ToolsetDoesNotExistError) Error() string {
	return fmt.Sprintf("toolset %s does not exist", e.Name)
}

func (e *ToolsetDoesNotExistError) Is(target error) bool {
	if target == nil {
		return false
	}
	if _, ok := target.(*ToolsetDoesNotExistError); ok {
		return true
	}
	return false
}

// NewToolsetDoesNotExistError creates a new ToolsetDoesNotExistError.
func NewToolsetDoesNotExistError(name string) *ToolsetDoesNotExistError {
	return &ToolsetDoesNotExistError{Name: name}
}

// ToolDoesNotExistError is returned when a tool is not found.
type ToolDoesNotExistError struct {
	Name string
}

func (e *ToolDoesNotExistError) Error() string {
	return fmt.Sprintf("tool %s does not exist", e.Name)
}

// NewToolDoesNotExistError creates a new ToolDoesNotExistError.
func NewToolDoesNotExistError(name string) *ToolDoesNotExistError {
	return &ToolDoesNotExistError{Name: name}
}
