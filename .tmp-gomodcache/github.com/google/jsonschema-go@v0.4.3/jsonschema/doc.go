// Copyright 2025 The JSON Schema Go Project Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

/*
Package jsonschema is an implementation of the [JSON Schema specification],
a JSON-based format for describing the structure of JSON data.
The package can be used to read schemas for code generation, and to validate
data using the draft 2020-12 and draft-07 specifications. Validation with
other drafts or custom meta-schemas is not supported.

Construct a [Schema] as you would any Go struct (for example, by writing
a struct literal), or unmarshal a JSON schema into a [Schema] in the usual
way (with [encoding/json], for instance). It can then be used for code
generation or other purposes without further processing.
You can also infer a schema from a Go struct.

# Resolution

A Schema can refer to other schemas, both inside and outside itself. These
references must be resolved before a schema can be used for validation.
Call [Schema.Resolve] to obtain a resolved schema (called a [Resolved]).
If the schema has external references, pass a [ResolveOptions] with a [Loader]
to load them. To validate default values in a schema, set
[ResolveOptions.ValidateDefaults] to true.

# Validation

Call [Resolved.Validate] to validate a JSON value. The value must be a
Go value that looks like the result of unmarshaling a JSON value into an
[any] or a struct. For example, the JSON value

	{"name": "Al", "scores": [90, 80, 100]}

could be represented as the Go value

	map[string]any{
		"name": "Al",
		"scores": []any{90, 80, 100},
	}

or as a value of this type:

	type Player struct {
		Name   string `json:"name"`
		Scores []int  `json:"scores"`
	}

# Inference

The [For] function returns a [Schema] describing the given Go type.
Each field in the struct becomes a property of the schema.
The values of "json" tags are respected: the field's property name is taken
from the tag, and fields omitted from the JSON are omitted from the schema as
well.
For example, `jsonschema.For[Player]()` returns this schema:

	{
	    "properties": {
	        "name": {
	            "type": "string"
	        },
	        "scores": {
	            "type": "array",
	            "items": {"type": "integer"}
	        }
	        "required": ["name", "scores"],
	        "additionalProperties": {"not": {}}
	    }
	}

Use the "jsonschema" struct tag to provide a description for the property:

	type Player struct {
		Name   string `json:"name" jsonschema:"player name"`
		Scores []int  `json:"scores" jsonschema:"scores of player's games"`
	}

# Deviations from the specification

Regular expressions are processed with Go's regexp package, which differs
from ECMA 262, most significantly in not supporting back-references.
See [this table of differences] for more.

The "format" keyword described in [section 7 of the validation spec] is recorded
in the Schema, but is ignored during validation.
It does not even produce [annotations].
Use the "pattern" keyword instead: it will work more reliably across JSON Schema
implementations. See [learnjsonschema.com] for more recommendations about "format".

The content keywords described in [section 8 of the validation spec]
are recorded in the schema, but ignored during validation.

# Controlling behavior changes

Minor and patch releases of this package may introduce behavior changes as part
of bug fixes or correctness improvements. To help manage the impact of such
changes, the package allows you to access previous behaviors using the
`JSONSCHEMAGODEBUG` environment variable. The available settings are listed
below; additional options may be introduced in future releases.

- **typeschemasnull**: When set to `"1"`, the inferred schema for slices will
*not* include the `null` type alongside the array type. It will also avoid
adding `null` to non-native pointer types (such as `time.Time`). This restores
the behavior from versions prior to v0.3.0. The default behavior is to include
`null` in these cases.

[JSON Schema specification]: https://json-schema.org
[section 7 of the validation spec]: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-00#rfc.section.7
[section 8 of the validation spec]: https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-00#rfc.section.8
[learnjsonschema.com]: https://www.learnjsonschema.com/2020-12/format-annotation/format/
[this table of differences]: https://github.com/dlclark/regexp2?tab=readme-ov-file#compare-regexp-and-regexp2
[annotations]: https://json-schema.org/draft/2020-12/json-schema-core#name-annotations
*/
package jsonschema
