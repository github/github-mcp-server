package inventory

// ToolInputError preserves a migrated handler's user-facing validation message
// when its checks move into an InputNormalizer.
type ToolInputError struct {
	Message string
}

func (err *ToolInputError) Error() string { return err.Message }
