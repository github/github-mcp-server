package github

import "fmt"

// normalizeInputAliases maps supported legacy parameter spellings to the
// canonical schema names before strict input validation. Supplying both forms
// is rejected so a caller cannot accidentally send conflicting values.
func normalizeInputAliases(args map[string]any, aliases map[string]string) error {
	for alias, canonical := range aliases {
		value, exists := args[alias]
		if !exists {
			continue
		}
		if _, canonicalExists := args[canonical]; canonicalExists {
			return fmt.Errorf("parameters %q and %q cannot both be provided", alias, canonical)
		}
		args[canonical] = value
		delete(args, alias)
	}
	return nil
}
