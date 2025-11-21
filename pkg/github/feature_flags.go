package github

// FeatureFlags defines runtime feature toggles that adjust tool behavior.
type FeatureFlags struct {
	LockdownMode bool
	// CSVFormat controls whether tools return a CSV response
	CSVFormat bool
	// TOONFormat controls whether tools return a TOON response
	TOONFormat bool
}
