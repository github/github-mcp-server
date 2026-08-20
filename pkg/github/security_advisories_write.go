package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v87/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var ghsaIDPattern = regexp.MustCompile(`(?i)^GHSA-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}$`)

func validateGHSAID(ghsaID string) error {
	if !ghsaIDPattern.MatchString(ghsaID) {
		return fmt.Errorf("invalid ghsaId format: must match GHSA-xxxx-xxxx-xxxx")
	}
	return nil
}

func normalizeGHSAID(ghsaID string) (string, error) {
	if err := validateGHSAID(ghsaID); err != nil {
		return "", err
	}
	if strings.HasPrefix(strings.ToLower(ghsaID), "ghsa-") {
		return "GHSA" + ghsaID[4:], nil
	}
	return ghsaID, nil
}

var securityAdvisoryPackageSchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"ecosystem": {
			Type:        "string",
			Description: "The package ecosystem.",
			Enum:        advisoryPackageEcosystemEnum,
		},
		"name": {
			Type:        "string",
			Description: "The package name.",
		},
	},
	Required: []string{"ecosystem", "name"},
}

var securityAdvisoryVulnerabilitySchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"package": securityAdvisoryPackageSchema,
		"vulnerable_version_range": {
			Type:        "string",
			Description: "The range of affected versions (for example, \"< 2.0.0\").",
		},
		"patched_versions": {
			Type:        "string",
			Description: "The version that patches the vulnerability.",
		},
		"vulnerable_functions": {
			Type:        "array",
			Description: "Functions in the package that are affected.",
			Items:       &jsonschema.Schema{Type: "string"},
		},
	},
	Required: []string{"package"},
}

var securityAdvisoryCreditSchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"login": {
			Type:        "string",
			Description: "The GitHub username of the credited user.",
		},
		"type": {
			Type:        "string",
			Description: "The credit type.",
			Enum:        []any{"analyst", "finder", "reporter", "coordinator", "remediation_developer", "remediation_reviewer", "remediation_verifier", "tool", "sponsor", "other"},
		},
	},
	Required: []string{"login", "type"},
}

type advisoryPackageRequest struct {
	Ecosystem string  `json:"ecosystem"`
	Name      *string `json:"name,omitempty"`
}

type advisoryVulnerabilityRequest struct {
	Package                advisoryPackageRequest `json:"package"`
	VulnerableVersionRange *string                `json:"vulnerable_version_range,omitempty"`
	PatchedVersions        *string                `json:"patched_versions,omitempty"`
	VulnerableFunctions    []string               `json:"vulnerable_functions,omitempty"`
}

type advisoryCreditRequest struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type createRepositorySecurityAdvisoryRequest struct {
	Summary          string                         `json:"summary"`
	Description      string                         `json:"description"`
	CVEID            *string                        `json:"cve_id,omitempty"`
	CWEIDs           []string                       `json:"cwe_ids,omitempty"`
	Severity         *string                        `json:"severity,omitempty"`
	CVSSVectorString *string                        `json:"cvss_vector_string,omitempty"`
	Vulnerabilities  []advisoryVulnerabilityRequest `json:"vulnerabilities"`
	Credits          []advisoryCreditRequest        `json:"credits,omitempty"`
	StartPrivateFork *bool                          `json:"start_private_fork,omitempty"`
}

type updateRepositorySecurityAdvisoryRequest struct {
	Summary          *string                        `json:"summary,omitempty"`
	Description      *string                        `json:"description,omitempty"`
	CVEID            *string                        `json:"cve_id,omitempty"`
	CWEIDs           []string                       `json:"cwe_ids,omitempty"`
	Severity         *string                        `json:"severity,omitempty"`
	CVSSVectorString *string                        `json:"cvss_vector_string,omitempty"`
	Vulnerabilities  []advisoryVulnerabilityRequest `json:"vulnerabilities,omitempty"`
	Credits          []advisoryCreditRequest        `json:"credits,omitempty"`
	State            *string                        `json:"state,omitempty"`
}

func parseAdvisoryVulnerabilities(args map[string]any, param string, required bool) ([]advisoryVulnerabilityRequest, error) {
	raw, ok := args[param]
	if !ok {
		if required {
			return nil, fmt.Errorf("missing required parameter: %s", param)
		}
		return nil, nil
	}
	if raw == nil {
		return nil, fmt.Errorf("invalid %s: at least one vulnerability must be provided", param)
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", param, err)
	}

	var vulns []advisoryVulnerabilityRequest
	if err := json.Unmarshal(data, &vulns); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", param, err)
	}
	if len(vulns) == 0 {
		return nil, fmt.Errorf("invalid %s: at least one vulnerability must be provided", param)
	}
	for i, vuln := range vulns {
		if vuln.Package.Ecosystem == "" {
			return nil, fmt.Errorf("invalid %s: vulnerabilities[%d].package.ecosystem is required", param, i)
		}
		if _, ok := validAdvisoryEcosystems[vuln.Package.Ecosystem]; !ok {
			return nil, fmt.Errorf("invalid %s: vulnerabilities[%d].package.ecosystem %q is invalid", param, i, vuln.Package.Ecosystem)
		}
		if vuln.Package.Name == nil || *vuln.Package.Name == "" {
			return nil, fmt.Errorf("invalid %s: vulnerabilities[%d].package.name is required", param, i)
		}
	}

	return vulns, nil
}

func parseAdvisoryCWEIDs(args map[string]any, param string) ([]string, error) {
	raw, ok := args[param]
	if !ok {
		return nil, nil
	}
	if raw == nil {
		return nil, fmt.Errorf("invalid %s: value must not be null", param)
	}
	cweIDs, err := OptionalStringArrayParam(args, param)
	if err != nil {
		return nil, err
	}
	if len(cweIDs) == 0 {
		return nil, fmt.Errorf("invalid %s: at least one CWE ID must be provided when %s is specified", param, param)
	}
	return cweIDs, nil
}

func parseAdvisoryCredits(args map[string]any, param string) ([]advisoryCreditRequest, error) {
	raw, ok := args[param]
	if !ok {
		return nil, nil
	}
	if raw == nil {
		return nil, fmt.Errorf("invalid %s: value must not be null", param)
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", param, err)
	}

	var credits []advisoryCreditRequest
	if err := json.Unmarshal(data, &credits); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", param, err)
	}
	if len(credits) == 0 {
		return nil, fmt.Errorf("invalid %s: at least one credit must be provided when %s is specified", param, param)
	}
	for i, credit := range credits {
		if credit.Login == "" {
			return nil, fmt.Errorf("invalid %s: credits[%d].login is required", param, i)
		}
		if credit.Type == "" {
			return nil, fmt.Errorf("invalid %s: credits[%d].type is required", param, i)
		}
		if _, ok := validAdvisoryCreditTypes[credit.Type]; !ok {
			return nil, fmt.Errorf("invalid %s: credits[%d].type %q is invalid", param, i, credit.Type)
		}
	}

	return credits, nil
}

func optionalStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func validateAdvisorySeverity(severity string) error {
	if severity == "" {
		return nil
	}
	if _, ok := validAdvisorySeverities[severity]; !ok {
		return fmt.Errorf("severity must be one of: low, medium, high, critical")
	}
	return nil
}

func validatePresentAdvisorySeverity(severity string) error {
	if severity == "" {
		return fmt.Errorf("severity must be one of: low, medium, high, critical")
	}
	return validateAdvisorySeverity(severity)
}

func validateAdvisoryState(state string) error {
	if state == "" {
		return nil
	}
	if _, ok := validAdvisoryStates[state]; !ok {
		return fmt.Errorf("state must be one of: draft, published, closed, triage")
	}
	return nil
}

func validatePresentAdvisoryState(state string) error {
	if state == "" {
		return fmt.Errorf("state must be one of: draft, published, closed, triage")
	}
	return validateAdvisoryState(state)
}

func validatePresentCVSSVectorString(cvssVectorString string) error {
	if cvssVectorString == "" {
		return fmt.Errorf("cvssVectorString must not be empty")
	}
	return nil
}

func validateSeverityOrCVSS(severity, cvssVectorString string, requireOne bool) error {
	hasSeverity := severity != ""
	hasCVSS := cvssVectorString != ""
	if hasSeverity {
		if err := validateAdvisorySeverity(severity); err != nil {
			return err
		}
	}
	if hasSeverity && hasCVSS {
		return fmt.Errorf("severity and cvssVectorString cannot both be set")
	}
	if requireOne && !hasSeverity && !hasCVSS {
		return fmt.Errorf("exactly one of severity or cvssVectorString must be provided")
	}
	return nil
}

func marshalRepositorySecurityAdvisoryResponse(advisory *github.SecurityAdvisory) (*mcp.CallToolResult, error) {
	r, err := json.Marshal(advisory)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal advisory: %w", err)
	}
	return utils.NewToolResultText(string(r)), nil
}

func repositorySecurityAdvisoryRequest(ctx context.Context, client *github.Client, method, owner, repo, ghsaID string, body any) (*github.SecurityAdvisory, *github.Response, error) {
	url := fmt.Sprintf("repos/%s/%s/security-advisories", owner, repo)
	if ghsaID != "" {
		normalizedGHSAID, err := normalizeGHSAID(ghsaID)
		if err != nil {
			return nil, nil, err
		}
		url = fmt.Sprintf("%s/%s", url, normalizedGHSAID)
	}

	req, err := client.NewRequest(ctx, method, url, body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	advisory := &github.SecurityAdvisory{}
	resp, err := client.Do(req, advisory)
	if err != nil {
		return nil, resp, err
	}

	return advisory, resp, nil
}

func CreateRepositorySecurityAdvisory(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataSecurityAdvisories,
		mcp.Tool{
			Name:        "create_repository_security_advisory",
			Description: t("TOOL_CREATE_REPOSITORY_SECURITY_ADVISORY_DESCRIPTION", "Create a draft repository security advisory. Exactly one of severity or cvssVectorString must be provided. When startPrivateFork is true, a temporary private fork is created for collaborating on a fix."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_CREATE_REPOSITORY_SECURITY_ADVISORY_USER_TITLE", "Create repository security advisory"),
				ReadOnlyHint:    false,
				OpenWorldHint:   jsonschema.Ptr(true),
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "The owner of the repository.",
					},
					"repo": {
						Type:        "string",
						Description: "The name of the repository.",
					},
					"summary": {
						Type:        "string",
						Description: "A short summary of the security advisory.",
					},
					"description": {
						Type:        "string",
						Description: "A detailed description of the security advisory.",
					},
					"vulnerabilities": {
						Type:        "array",
						Description: "Affected products and version ranges.",
						Items:       securityAdvisoryVulnerabilitySchema,
					},
					"cveId": {
						Type:        "string",
						Description: "The CVE ID to assign to the advisory.",
					},
					"cweIds": {
						Type:        "array",
						Description: "Common Weakness Enumeration IDs (for example, [\"CWE-79\"]).",
						Items:       &jsonschema.Schema{Type: "string"},
					},
					"severity": {
						Type:        "string",
						Description: "The severity of the advisory. Exactly one of severity or cvssVectorString is required.",
						Enum:        []any{"low", "medium", "high", "critical"},
					},
					"cvssVectorString": {
						Type:        "string",
						Description: "The CVSS vector string for the advisory. Exactly one of severity or cvssVectorString is required.",
					},
					"credits": {
						Type:        "array",
						Description: "Users credited for the advisory.",
						Items:       securityAdvisoryCreditSchema,
					},
					"startPrivateFork": {
						Type:        "boolean",
						Description: "Whether to create a temporary private fork for collaborating on a fix.",
					},
				},
				Required: []string{"owner", "repo", "summary", "description", "vulnerabilities"},
			},
		},
		[]scopes.Scope{scopes.SecurityEvents},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			summary, err := RequiredParam[string](args, "summary")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			description, err := RequiredParam[string](args, "description")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			vulnerabilities, err := parseAdvisoryVulnerabilities(args, "vulnerabilities", true)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			cveID, err := OptionalParam[string](args, "cveId")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			cweIDs, err := parseAdvisoryCWEIDs(args, "cweIds")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			severity, err := OptionalParam[string](args, "severity")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			cvssVectorString, err := OptionalParam[string](args, "cvssVectorString")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			credits, err := parseAdvisoryCredits(args, "credits")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			startPrivateFork, err := OptionalParam[bool](args, "startPrivateFork")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if err := validateSeverityOrCVSS(severity, cvssVectorString, true); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			requestBody := createRepositorySecurityAdvisoryRequest{
				Summary:          summary,
				Description:      description,
				CVEID:            optionalStringPtr(cveID),
				CWEIDs:           cweIDs,
				Severity:         optionalStringPtr(severity),
				CVSSVectorString: optionalStringPtr(cvssVectorString),
				Vulnerabilities:  vulnerabilities,
				Credits:          credits,
			}
			if _, ok := args["startPrivateFork"]; ok {
				requestBody.StartPrivateFork = &startPrivateFork
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			advisory, resp, err := repositorySecurityAdvisoryRequest(ctx, client, http.MethodPost, owner, repo, "", requestBody)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create repository security advisory", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create repository security advisory", resp, body), nil, nil
			}

			result, err := marshalRepositorySecurityAdvisoryResponse(advisory)
			if err != nil {
				return nil, nil, err
			}
			return result, nil, nil
		},
	)
}

func UpdateRepositorySecurityAdvisory(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataSecurityAdvisories,
		mcp.Tool{
			Name:        "update_repository_security_advisory",
			Description: t("TOOL_UPDATE_REPOSITORY_SECURITY_ADVISORY_DESCRIPTION", "Update a repository security advisory, including publishing it. Severity and cvssVectorString cannot both be set. Omit credits and cweIds to leave them unchanged; empty arrays are rejected and clearing these fields is not supported."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_REPOSITORY_SECURITY_ADVISORY_USER_TITLE", "Update repository security advisory"),
				ReadOnlyHint:    false,
				OpenWorldHint:   jsonschema.Ptr(true),
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "The owner of the repository.",
					},
					"repo": {
						Type:        "string",
						Description: "The name of the repository.",
					},
					"ghsaId": {
						Type:        "string",
						Description: "GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx).",
					},
					"summary": {
						Type:        "string",
						Description: "A short summary of the security advisory.",
					},
					"description": {
						Type:        "string",
						Description: "A detailed description of the security advisory.",
					},
					"vulnerabilities": {
						Type:        "array",
						Description: "Affected products and version ranges.",
						Items:       securityAdvisoryVulnerabilitySchema,
					},
					"cveId": {
						Type:        "string",
						Description: "The CVE ID to assign to the advisory.",
					},
					"cweIds": {
						Type:        "array",
						Description: "Common Weakness Enumeration IDs (for example, [\"CWE-79\"]). Omit to leave unchanged; empty arrays are rejected and clearing this field is not supported.",
						Items:       &jsonschema.Schema{Type: "string"},
					},
					"severity": {
						Type:        "string",
						Description: "The severity of the advisory. Cannot be set together with cvssVectorString.",
						Enum:        []any{"low", "medium", "high", "critical"},
					},
					"cvssVectorString": {
						Type:        "string",
						Description: "The CVSS vector string for the advisory. Cannot be set together with severity.",
					},
					"credits": {
						Type:        "array",
						Description: "Users credited for the advisory. Omit to leave unchanged; empty arrays are rejected and clearing this field is not supported.",
						Items:       securityAdvisoryCreditSchema,
					},
					"state": {
						Type:        "string",
						Description: "The advisory state. Set to \"published\" to publish the advisory.",
						Enum:        []any{"draft", "published", "closed", "triage"},
					},
				},
				Required: []string{"owner", "repo", "ghsaId"},
			},
		},
		[]scopes.Scope{scopes.SecurityEvents},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ghsaID, err := RequiredParam[string](args, "ghsaId")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ghsaID, err = normalizeGHSAID(ghsaID)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			summary, err := OptionalParam[string](args, "summary")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			description, err := OptionalParam[string](args, "description")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			vulnerabilities, err := parseAdvisoryVulnerabilities(args, "vulnerabilities", false)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			cveID, err := OptionalParam[string](args, "cveId")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			cweIDs, err := parseAdvisoryCWEIDs(args, "cweIds")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			severity, err := OptionalParam[string](args, "severity")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			cvssVectorString, err := OptionalParam[string](args, "cvssVectorString")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			credits, err := parseAdvisoryCredits(args, "credits")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if _, ok := args["severity"]; ok {
				if err := validatePresentAdvisorySeverity(severity); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
			}
			if _, ok := args["cvssVectorString"]; ok {
				if err := validatePresentCVSSVectorString(cvssVectorString); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
			}
			if _, ok := args["state"]; ok {
				if err := validatePresentAdvisoryState(state); err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
			}
			if err := validateSeverityOrCVSS(severity, cvssVectorString, false); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			requestBody := updateRepositorySecurityAdvisoryRequest{}
			hasUpdate := false
			if _, ok := args["summary"]; ok {
				requestBody.Summary = &summary
				hasUpdate = true
			}
			if _, ok := args["description"]; ok {
				requestBody.Description = &description
				hasUpdate = true
			}
			if _, ok := args["vulnerabilities"]; ok {
				requestBody.Vulnerabilities = vulnerabilities
				hasUpdate = true
			}
			if _, ok := args["cveId"]; ok {
				requestBody.CVEID = &cveID
				hasUpdate = true
			}
			if _, ok := args["cweIds"]; ok {
				requestBody.CWEIDs = cweIDs
				hasUpdate = true
			}
			if _, ok := args["severity"]; ok {
				requestBody.Severity = &severity
				hasUpdate = true
			}
			if _, ok := args["cvssVectorString"]; ok {
				requestBody.CVSSVectorString = &cvssVectorString
				hasUpdate = true
			}
			if _, ok := args["credits"]; ok {
				requestBody.Credits = credits
				hasUpdate = true
			}
			if _, ok := args["state"]; ok {
				requestBody.State = &state
				hasUpdate = true
			}

			if !hasUpdate {
				return utils.NewToolResultError("at least one of summary, description, vulnerabilities, cveId, cweIds, severity, cvssVectorString, credits, or state must be provided for update"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			advisory, resp, err := repositorySecurityAdvisoryRequest(ctx, client, http.MethodPatch, owner, repo, ghsaID, requestBody)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update repository security advisory", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to update repository security advisory", resp, body), nil, nil
			}

			result, err := marshalRepositorySecurityAdvisoryResponse(advisory)
			if err != nil {
				return nil, nil, err
			}
			return result, nil, nil
		},
	)
}

func RequestCVEForRepositorySecurityAdvisory(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataSecurityAdvisories,
		mcp.Tool{
			Name:        "request_cve_for_repository_security_advisory",
			Description: t("TOOL_REQUEST_CVE_FOR_REPOSITORY_SECURITY_ADVISORY_DESCRIPTION", "Request a CVE ID from GitHub for a draft repository security advisory."),
			Annotations: &mcp.ToolAnnotations{
				Title:            t("TOOL_REQUEST_CVE_FOR_REPOSITORY_SECURITY_ADVISORY_USER_TITLE", "Request CVE for repository security advisory"),
				ReadOnlyHint:     false,
				DestructiveHint:  jsonschema.Ptr(true),
				OpenWorldHint:    jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "The owner of the repository.",
					},
					"repo": {
						Type:        "string",
						Description: "The name of the repository.",
					},
					"ghsaId": {
						Type:        "string",
						Description: "GitHub Security Advisory ID (format: GHSA-xxxx-xxxx-xxxx).",
					},
				},
				Required: []string{"owner", "repo", "ghsaId"},
			},
		},
		[]scopes.Scope{scopes.SecurityEvents},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ghsaID, err := RequiredParam[string](args, "ghsaId")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ghsaID, err = normalizeGHSAID(ghsaID)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.SecurityAdvisories.RequestCVE(ctx, owner, repo, ghsaID)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to request CVE for repository security advisory", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to request CVE for repository security advisory", resp, body), nil, nil
			}

			return utils.NewToolResultText("CVE request submitted successfully"), nil, nil
		},
	)
}
