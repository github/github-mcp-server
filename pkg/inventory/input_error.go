package inventory

// ToolInputError represents an argument validation error whose message should
// be shown to the tool caller without additional wrapper text.
type ToolInputError struct {
	Message string
}

// Error implements error.
func (err *ToolInputError) Error() string {
	return err.Message
}
