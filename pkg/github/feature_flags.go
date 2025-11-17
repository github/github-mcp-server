package github

// FeatureFlags defines runtime feature toggles that adjust tool behavior.
type FeatureFlags struct {
	LockdownMode bool
	// JSONFormat controls whether tools return a JSON response
	JSONFormat bool
	// TOONFormat controls whether tools return a TOON response
	TOONFormat bool
}
