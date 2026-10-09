package github

import (
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
)

func constrainMethodOutput(schema *jsonschema.Schema, payloadByMethod map[string]string, optionalPayloads ...string) {
	schema.Required = []string{"method"}
	for _, method := range schema.Properties["method"].Enum {
		payload := payloadByMethod[method.(string)]
		variant := &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{
			"method": {Enum: []any{method}},
		}}
		if !slices.Contains(optionalPayloads, payload) {
			variant.Required = []string{payload}
		}
		for _, other := range payloadByMethod {
			if other != payload {
				variant.Properties[other] = forbiddenIssueVariantProperty()
			}
		}
		schema.OneOf = append(schema.OneOf, variant)
	}
}
