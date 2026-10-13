package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"strings"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MinimalLabel struct {
	Name        string `json:"name"`
	Color       string `json:"color" jsonschema:"Hex RGB without leading #."`
	Description string `json:"description"`
}

type ListLabelsOutput struct {
	Labels     []MinimalLabel `json:"labels"`
	TotalCount int            `json:"totalCount" jsonschema:"Total labels in the repository."`
}

type LabelWriteOutput struct {
	Method  string `json:"method"`
	Message string `json:"message"`
}

// CustomPropertyOutputValue is the API's string/string-array/null union.
type CustomPropertyOutputValue struct {
	raw json.RawMessage
}

func (v *CustomPropertyOutputValue) UnmarshalJSON(raw []byte) error {
	schema, err := customPropertyValueSchema().Properties["value"].Resolve(nil)
	if err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	if err := schema.Validate(value); err != nil {
		return err
	}
	v.raw = append(json.RawMessage(nil), raw...)
	return nil
}

func (v CustomPropertyOutputValue) MarshalJSON() ([]byte, error) {
	if len(v.raw) == 0 {
		return []byte("null"), nil
	}
	return v.raw, nil
}

func (v CustomPropertyOutputValue) IsZero() bool {
	return len(v.raw) == 0
}

type CustomPropertyValueOutput struct {
	PropertyName string                    `json:"property_name"`
	Value        CustomPropertyOutputValue `json:"value"`
}

type CustomPropertyDefinitionOutput struct {
	PropertyName          string                    `json:"property_name"`
	ValueType             string                    `json:"value_type"`
	Required              *bool                     `json:"required,omitempty"`
	DefaultValue          CustomPropertyOutputValue `json:"default_value,omitzero"`
	Description           *string                   `json:"description,omitempty"`
	AllowedValues         []string                  `json:"allowed_values,omitzero"`
	ValuesEditableBy      *string                   `json:"values_editable_by,omitempty"`
	RequireExplicitValues *bool                     `json:"require_explicit_values,omitempty"`
}

type CustomPropertiesOutput struct {
	Method      string                            `json:"method"`
	Level       string                            `json:"level"`
	Values      *[]CustomPropertyValueOutput      `json:"values,omitempty"`
	Definitions *[]CustomPropertyDefinitionOutput `json:"definitions,omitempty"`
	Message     string                            `json:"message,omitempty"`
}

type RulesetBypassActorOutput struct {
	ActorID    *int64 `json:"actor_id,omitempty"`
	ActorType  string `json:"actor_type"`
	BypassMode string `json:"bypass_mode"`
}

type RulesetNameConditionOutput struct {
	Include   []string `json:"include"`
	Exclude   []string `json:"exclude"`
	Protected *bool    `json:"protected,omitempty"`
}

type RulesetIDConditionOutput struct {
	RepositoryIDs   []int64 `json:"repository_ids,omitempty"`
	OrganizationIDs []int64 `json:"organization_ids,omitempty"`
}

type RulesetPropertyTargetOutput struct {
	Name           string   `json:"name"`
	PropertyValues []string `json:"property_values"`
	Source         *string  `json:"source,omitempty"`
}

type RulesetPropertyConditionOutput struct {
	Include []RulesetPropertyTargetOutput `json:"include"`
	Exclude []RulesetPropertyTargetOutput `json:"exclude"`
}

type RulesetConditionsOutput struct {
	RefName              *RulesetNameConditionOutput     `json:"ref_name,omitempty"`
	RepositoryID         *RulesetIDConditionOutput       `json:"repository_id,omitempty"`
	RepositoryName       *RulesetNameConditionOutput     `json:"repository_name,omitempty"`
	RepositoryProperty   *RulesetPropertyConditionOutput `json:"repository_property,omitempty"`
	OrganizationID       *RulesetIDConditionOutput       `json:"organization_id,omitempty"`
	OrganizationName     *RulesetNameConditionOutput     `json:"organization_name,omitempty"`
	OrganizationProperty *RulesetPropertyConditionOutput `json:"organization_property,omitempty"`
}

type GovernanceRuleOutput struct {
	Type string `json:"type"`
	// Parameters are the only passthrough: rule-specific, user-configured JSON.
	Parameters        json.RawMessage `json:"parameters,omitempty"`
	RulesetID         *int64          `json:"ruleset_id,omitempty"`
	RulesetSource     string          `json:"ruleset_source,omitempty"`
	RulesetSourceType string          `json:"ruleset_source_type,omitempty"`
}

type RulesetOutput struct {
	ID                   *int64                     `json:"id,omitempty"`
	Name                 string                     `json:"name"`
	Target               string                     `json:"target,omitempty"`
	SourceType           string                     `json:"source_type,omitempty"`
	Source               string                     `json:"source"`
	Enforcement          string                     `json:"enforcement"`
	BypassActors         []RulesetBypassActorOutput `json:"bypass_actors,omitzero"`
	CurrentUserCanBypass string                     `json:"current_user_can_bypass,omitempty"`
	Conditions           *RulesetConditionsOutput   `json:"conditions,omitempty"`
	Rules                []GovernanceRuleOutput     `json:"rules,omitzero"`
	CreatedAt            string                     `json:"created_at,omitempty" jsonschema:"RFC3339 creation time."`
	UpdatedAt            string                     `json:"updated_at,omitempty" jsonschema:"RFC3339 last update time."`
}

type RuleEvaluationOutput struct {
	RuleSource  *RuleSourceOutput `json:"rule_source,omitempty"`
	Enforcement string            `json:"enforcement,omitempty"`
	Result      string            `json:"result"`
	RuleType    string            `json:"rule_type"`
	Details     string            `json:"details,omitempty"`
}

type RuleSourceOutput struct {
	Type string `json:"type"`
	ID   *int64 `json:"id"`
	Name string `json:"name,omitempty"`
}

type RuleSuiteOutput struct {
	ID               int64                  `json:"id"`
	ActorID          *int64                 `json:"actor_id,omitempty"`
	ActorName        string                 `json:"actor_name,omitempty"`
	BeforeSHA        string                 `json:"before_sha,omitempty"`
	AfterSHA         string                 `json:"after_sha,omitempty"`
	Ref              string                 `json:"ref,omitempty"`
	PushedAt         string                 `json:"pushed_at,omitempty" jsonschema:"RFC3339 push time."`
	Result           string                 `json:"result"`
	EvaluationResult *string                `json:"evaluation_result,omitempty" jsonschema:"Result of rules evaluated without enforcement, distinct from the enforced result."`
	RepositoryID     *int64                 `json:"repository_id,omitempty"`
	RepositoryName   string                 `json:"repository_name,omitempty"`
	Evaluations      []RuleEvaluationOutput `json:"rule_evaluations,omitempty"`
}

type RulesetReadOutput struct {
	Method     string                  `json:"method"`
	Level      string                  `json:"level"`
	Ruleset    *RulesetOutput          `json:"ruleset,omitempty"`
	Rulesets   *[]RulesetOutput        `json:"rulesets,omitempty"`
	TotalCount *int                    `json:"total_count,omitempty"`
	Rules      *[]GovernanceRuleOutput `json:"rules,omitempty"`
	RuleSuites *[]RuleSuiteOutput      `json:"rule_suites,omitempty"`
	RuleSuite  *RuleSuiteOutput        `json:"rule_suite,omitempty"`
}

func typedGovernanceSchema[T any]() *jsonschema.Schema {
	definitions := map[string]reflect.Type{
		"ruleset":           reflect.TypeFor[RulesetOutput](),
		"rule":              reflect.TypeFor[GovernanceRuleOutput](),
		"ruleSuite":         reflect.TypeFor[RuleSuiteOutput](),
		"nameCondition":     reflect.TypeFor[RulesetNameConditionOutput](),
		"idCondition":       reflect.TypeFor[RulesetIDConditionOutput](),
		"propertyCondition": reflect.TypeFor[RulesetPropertyConditionOutput](),
	}
	options := &jsonschema.ForOptions{
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[CustomPropertyOutputValue](): customPropertyValueSchema().Properties["value"],
			reflect.TypeFor[json.RawMessage](): {
				Type:        "object",
				Description: "User-configured parameters for this rule type.",
			},
		},
	}
	for name, typ := range definitions {
		if typ != reflect.TypeFor[T]() {
			options.TypeSchemas[typ] = &jsonschema.Schema{Type: "object", Ref: "#/$defs/" + name}
		}
	}
	schema, err := jsonschema.For[T](options)
	if err != nil {
		panic(err)
	}
	governanceStateSchema(schema, reflect.TypeFor[T]())
	schema.Defs = make(map[string]*jsonschema.Schema)
	for {
		added := false
		encoded, err := json.Marshal(schema)
		if err != nil {
			panic(err)
		}
		for name, typ := range definitions {
			if schema.Defs[name] != nil || !strings.Contains(string(encoded), `#/$defs/`+name) {
				continue
			}
			definitionOptions := &jsonschema.ForOptions{TypeSchemas: maps.Clone(options.TypeSchemas)}
			delete(definitionOptions.TypeSchemas, typ)
			definition, err := jsonschema.ForType(typ, definitionOptions)
			if err != nil {
				panic(err)
			}
			governanceStateSchema(definition, typ)
			schema.Defs[name] = definition
			added = true
		}
		if !added {
			break
		}
	}
	return forbidOmittedNulls(schema)
}

func governanceStateSchema(schema *jsonschema.Schema, typ reflect.Type) {
	switch typ {
	case reflect.TypeFor[RulesetOutput]():
		schema.Properties["enforcement"].Enum = []any{"", "disabled", "active", "evaluate"}
		schema.Properties["target"].Enum = []any{"branch", "tag", "push", "repository"}
		schema.Properties["source_type"].Enum = []any{"Repository", "Organization", "Enterprise"}
		for _, name := range []string{"created_at", "updated_at"} {
			schema.Properties[name].Format = "date-time"
		}
	case reflect.TypeFor[RuleSuiteOutput]():
		schema.Properties["result"].Enum = []any{"pass", "fail", "bypass"}
		schema.Properties["pushed_at"].Format = "date-time"
	}
}

func labelOutputSchema() *jsonschema.Schema      { return typedGovernanceSchema[MinimalLabel]() }
func listLabelsOutputSchema() *jsonschema.Schema { return typedGovernanceSchema[ListLabelsOutput]() }
func labelWriteOutputSchema() *jsonschema.Schema {
	schema := typedGovernanceSchema[LabelWriteOutput]()
	schema.Properties["method"].Enum = []any{"create", "update", "delete"}
	return schema
}
func customPropertiesReadOutputSchema() *jsonschema.Schema {
	schema := typedGovernanceSchema[CustomPropertiesOutput]()
	schema.Properties["method"].Enum = []any{"read"}
	schema.Properties["level"].Enum = []any{"repository", "organization", "enterprise"}
	delete(schema.Properties, "message")
	customPropertyOutputStates(schema)
	return schema
}
func customPropertiesWriteOutputSchema() *jsonschema.Schema {
	schema := typedGovernanceSchema[CustomPropertiesOutput]()
	schema.Properties["method"].Enum = []any{"write"}
	schema.Properties["level"].Enum = []any{"repository", "organization", "enterprise"}
	delete(schema.Properties, "values")
	customPropertyOutputStates(schema)
	return schema
}

func customPropertyOutputStates(schema *jsonschema.Schema) {
	definition := schema.Properties["definitions"].Items
	definition.Properties["value_type"].Enum = []any{"", "string", "single_select", "multi_select", "true_false", "url"}
	definition.Properties["values_editable_by"].Enum = []any{"org_actors", "org_and_repo_actors"}
}
func rulesetReadOutputSchema() *jsonschema.Schema {
	schema := typedGovernanceSchema[RulesetReadOutput]()
	schema.Properties["method"].Enum = []any{"get", "list", "get_rules_for_branch", "list_rule_suites", "get_rule_suite"}
	schema.Properties["level"].Enum = []any{"repository", "organization", "enterprise"}
	return schema
}
func createdRulesetOutputSchema() *jsonschema.Schema { return typedGovernanceSchema[RulesetOutput]() }

func decodeGovernanceJSON[T any](raw []byte, _ map[string]json.RawMessage) (T, error) {
	var output T
	err := json.Unmarshal(raw, &output)
	return output, err
}

func governanceArgument(args map[string]json.RawMessage, name string) string {
	var value string
	_ = json.Unmarshal(args[name], &value)
	return value
}

func decodeCustomProperties(method string) func([]byte, map[string]json.RawMessage) (CustomPropertiesOutput, error) {
	return func(raw []byte, args map[string]json.RawMessage) (CustomPropertiesOutput, error) {
		out := CustomPropertiesOutput{Method: method, Level: governanceArgument(args, "level")}
		if out.Level == "repository" {
			if method == "write" {
				out.Message = string(raw)
				return out, nil
			}
			values := []CustomPropertyValueOutput{}
			err := json.Unmarshal(raw, &values)
			if values == nil {
				values = []CustomPropertyValueOutput{}
			}
			out.Values = new(values)
			return out, err
		}
		definitions := []CustomPropertyDefinitionOutput{}
		err := json.Unmarshal(raw, &definitions)
		if definitions == nil {
			definitions = []CustomPropertyDefinitionOutput{}
		}
		out.Definitions = new(definitions)
		return out, err
	}
}

func decodeRulesetRead(raw []byte, args map[string]json.RawMessage) (RulesetReadOutput, error) {
	out := RulesetReadOutput{
		Method: strings.ToLower(governanceArgument(args, "method")),
		Level:  governanceArgument(args, "level"),
	}
	var err error
	switch out.Method {
	case "get":
		err = json.Unmarshal(raw, &out.Ruleset)
	case "list":
		if out.Level == "enterprise" {
			err = json.Unmarshal(raw, &out)
			if out.Rulesets == nil || *out.Rulesets == nil {
				out.Rulesets = new([]RulesetOutput{})
			}
		} else {
			rulesets := []RulesetOutput{}
			err = json.Unmarshal(raw, &rulesets)
			if rulesets == nil {
				rulesets = []RulesetOutput{}
			}
			out.Rulesets = new(rulesets)
		}
	case "get_rules_for_branch":
		// go-github's legacy BranchRules text groups rules by Go field name.
		var groups map[string][]GovernanceRuleOutput
		err = json.Unmarshal(raw, &groups)
		if err == nil {
			rules := []GovernanceRuleOutput{}
			for _, name := range governanceBranchRuleNames {
				for _, rule := range groups[name.field] {
					rule.Type = name.kind
					rules = append(rules, rule)
				}
			}
			out.Rules = new(rules)
		}
	case "list_rule_suites":
		suites := []RuleSuiteOutput{}
		err = json.Unmarshal(raw, &suites)
		if suites == nil {
			suites = []RuleSuiteOutput{}
		}
		out.RuleSuites = new(suites)
	case "get_rule_suite":
		err = json.Unmarshal(raw, &out.RuleSuite)
	default:
		err = fmt.Errorf("unexpected successful ruleset method %q", out.Method)
	}
	return out, err
}

var governanceBranchRuleNames = []struct{ field, kind string }{
	{"Creation", "creation"}, {"Update", "update"}, {"Deletion", "deletion"},
	{"RequiredLinearHistory", "required_linear_history"}, {"MergeQueue", "merge_queue"},
	{"RequiredDeployments", "required_deployments"}, {"RequiredSignatures", "required_signatures"},
	{"PullRequest", "pull_request"}, {"RequiredStatusChecks", "required_status_checks"},
	{"NonFastForward", "non_fast_forward"}, {"CommitMessagePattern", "commit_message_pattern"},
	{"CommitAuthorEmailPattern", "commit_author_email_pattern"}, {"CommitterEmailPattern", "committer_email_pattern"},
	{"BranchNamePattern", "branch_name_pattern"}, {"TagNamePattern", "tag_name_pattern"},
	{"Workflows", "workflows"}, {"CodeScanning", "code_scanning"}, {"CopilotCodeReview", "copilot_code_review"},
	{"FileExtensionRestriction", "file_extension_restriction"}, {"FilePathRestriction", "file_path_restriction"},
	{"MaxFilePathLength", "max_file_path_length"}, {"MaxFileSize", "max_file_size"},
}

// governanceLevelRoutingFields maps each "level" value accepted by the
// level-routed governance tools (custom_properties_read/write,
// repository_ruleset_read, create_repository_ruleset) to the input field(s)
// the legacy handler reads only at that level. Fields unused at the call's
// level are legacy no-ops: the handler never looks at them, so a malformed
// (wrong-typed) unused field must not fail validation before level routing
// even runs.
var governanceLevelRoutingFields = map[string][]string{
	"repository":   {"owner", "repo"},
	"organization": {"org"},
	"enterprise":   {"enterprise"},
}

// normalizeGovernanceLevelRouting strips level-routing fields that are unused
// for the call's "level" before SDK schema validation runs, so an unused
// field's type (or presence) can never produce a validation error the legacy
// handler itself would never raise. It never touches used fields, and it
// leaves arguments untouched whenever "level" is missing, non-string, or not
// a recognized value, so the legacy handler's own "unknown level" error still
// fires exactly as before.
func normalizeGovernanceLevelRouting(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]json.RawMessage
	if err := json.Unmarshal(raw, &args); err != nil {
		return raw, nil
	}
	levelRaw, ok := args["level"]
	if !ok {
		return raw, nil
	}
	var level string
	if err := json.Unmarshal(levelRaw, &level); err != nil {
		return raw, nil
	}
	used, known := governanceLevelRoutingFields[level]
	if !known {
		return raw, nil
	}
	keep := make(map[string]bool, len(used))
	for _, field := range used {
		keep[field] = true
	}
	changed := false
	for _, fields := range governanceLevelRoutingFields {
		for _, field := range fields {
			if keep[field] {
				continue
			}
			if _, present := args[field]; present {
				delete(args, field)
				changed = true
			}
		}
	}
	if !changed {
		return raw, nil
	}
	return json.Marshal(args)
}

func typedGovernanceTool[Out any](
	legacy inventory.ServerTool,
	outputSchema *jsonschema.Schema,
	decode func([]byte, map[string]json.RawMessage) (Out, error),
	normalizers ...inventory.InputNormalizer,
) inventory.ServerTool {
	tool := legacy.Tool
	tool.OutputSchema = outputSchema
	// Legacy handlers validate only the fields consumed by the selected route.
	// Keep discovery unchanged while preserving their accepted argument forms.
	typed := NewToolWithSchemaOptions[map[string]json.RawMessage, *Out](
		legacy.Toolset, tool, legacy.ScopeAccess,
		inventory.TypedSchemaOptions{ValidationInputSchema: &jsonschema.Schema{Type: "object"}},
		func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args map[string]json.RawMessage) (*mcp.CallToolResult, *Out, error) {
			result, err := legacy.Handler(deps)(ctx, req)
			if err != nil || result == nil || result.IsError {
				return result, nil, err
			}
			if len(result.Content) != 1 {
				return nil, nil, fmt.Errorf("expected one legacy %s text result", tool.Name)
			}
			content, ok := result.Content[0].(*mcp.TextContent)
			if !ok {
				return nil, nil, fmt.Errorf("expected legacy %s text content", tool.Name)
			}
			output, err := decode([]byte(content.Text), args)
			if err != nil {
				return nil, nil, fmt.Errorf("decode %s output: %w", tool.Name, err)
			}
			return result, new(output), nil
		},
		normalizers...,
	)
	typed.FeatureRule = legacy.FeatureRule
	typed.Enabled = legacy.Enabled
	typed.MinimumProtocolVersion = legacy.MinimumProtocolVersion
	typed.RequiredElicitationMode = legacy.RequiredElicitationMode
	return typed
}
