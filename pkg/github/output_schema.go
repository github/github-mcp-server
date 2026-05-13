package github

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// MustOutputSchema infers an object output schema for T or panics during tool initialization.
func MustOutputSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		var zero T
		panic(fmt.Sprintf("failed to infer output schema for %T: %v", zero, err))
	}
	if schema.Type != "object" {
		var zero T
		panic(fmt.Sprintf("output schema for %T must have type object, got %q", zero, schema.Type))
	}
	return schema
}
