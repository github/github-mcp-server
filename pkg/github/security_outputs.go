package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/google/go-github/v92/github"
)

type GetCodeQualityFindingInput struct {
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	FindingNumber int    `json:"findingNumber"`
}

type GetSecurityAlertInput struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	AlertNumber int    `json:"alertNumber"`
}

type ListCodeScanningAlertsInput struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	State    string `json:"state,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Severity string `json:"severity,omitempty"`
	ToolName string `json:"tool_name,omitempty"`
	Page     *int   `json:"page,omitempty"`
	PerPage  *int   `json:"perPage,omitempty"`
}

type ListSecretScanningAlertsInput struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	State      string `json:"state,omitempty"`
	SecretType string `json:"secret_type,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	Page       *int   `json:"page,omitempty"`
	PerPage    *int   `json:"perPage,omitempty"`
}

type ListDependabotAlertsInput struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	State    string `json:"state,omitempty"`
	Severity string `json:"severity,omitempty"`
	PerPage  *int   `json:"perPage,omitempty"`
	After    string `json:"after,omitempty"`
}

type ListGlobalSecurityAdvisoriesInput struct {
	GHSAID      string   `json:"ghsaId,omitempty"`
	Type        string   `json:"type,omitempty"`
	CVEID       string   `json:"cveId,omitempty"`
	Ecosystem   string   `json:"ecosystem,omitempty"`
	Severity    string   `json:"severity,omitempty"`
	CWEs        []string `json:"cwes,omitempty"`
	IsWithdrawn bool     `json:"isWithdrawn,omitempty"`
	Affects     string   `json:"affects,omitempty"`
	Published   string   `json:"published,omitempty"`
	Updated     string   `json:"updated,omitempty"`
	Modified    string   `json:"modified,omitempty"`
}

type ListRepositorySecurityAdvisoriesInput struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	Direction string `json:"direction,omitempty"`
	Sort      string `json:"sort,omitempty"`
	State     string `json:"state,omitempty"`
}

type GetGlobalSecurityAdvisoryInput struct {
	GHSAID string `json:"ghsaId"`
}

type ListOrgRepositorySecurityAdvisoriesInput struct {
	Org       string `json:"org"`
	Direction string `json:"direction,omitempty"`
	Sort      string `json:"sort,omitempty"`
	State     string `json:"state,omitempty"`
}

type CodeQualityRuleOutput struct {
	ID          *string `json:"id,omitempty"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Help        *string `json:"help,omitempty"`
	Severity    *string `json:"severity,omitempty"`
	Category    *string `json:"category,omitempty"`
}

type CodeQualityLocationOutput struct {
	Path        *string `json:"path,omitempty"`
	StartLine   *int    `json:"start_line,omitempty"`
	StartColumn *int    `json:"start_column,omitempty"`
	EndLine     *int    `json:"end_line,omitempty"`
	EndColumn   *int    `json:"end_column,omitempty"`
}

type CodeQualityMessageOutput struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown"`
}

type CodeQualityFindingOutput struct {
	Number    *int                       `json:"number,omitempty"`
	State     *string                    `json:"state,omitempty"`
	Rule      *CodeQualityRuleOutput     `json:"rule,omitempty"`
	Location  *CodeQualityLocationOutput `json:"location,omitempty"`
	Message   *CodeQualityMessageOutput  `json:"message,omitempty"`
	CreatedAt *time.Time                 `json:"created_at,omitempty" jsonschema:"Finding creation time (RFC 3339)."`
}

type CodeScanningRuleOutput struct {
	ID                    *string `json:"id,omitempty"`
	Severity              *string `json:"severity,omitempty"`
	Description           *string `json:"description,omitempty"`
	Name                  *string `json:"name,omitempty"`
	SecuritySeverityLevel *string `json:"security_severity_level,omitempty" jsonschema:"Security impact level, distinct from rule diagnostic severity."`
	Help                  *string `json:"help,omitempty"`
}

type CodeScanningLocationOutput struct {
	Path        *string `json:"path,omitempty"`
	StartLine   *int    `json:"start_line,omitempty"`
	EndLine     *int    `json:"end_line,omitempty"`
	StartColumn *int    `json:"start_column,omitempty"`
	EndColumn   *int    `json:"end_column,omitempty"`
}

type CodeScanningInstanceOutput struct {
	Ref       *string                     `json:"ref,omitempty"`
	State     *string                     `json:"state,omitempty"`
	CommitSHA *string                     `json:"commit_sha,omitempty"`
	Message   *string                     `json:"message,omitempty"`
	Location  *CodeScanningLocationOutput `json:"location,omitempty"`
	HTMLURL   *string                     `json:"html_url,omitempty"`
}

type CodeScanningAlertOutput struct {
	Number             *int                        `json:"number,omitempty"`
	State              *string                     `json:"state,omitempty"`
	HTMLURL            *string                     `json:"html_url,omitempty"`
	Rule               *CodeScanningRuleOutput     `json:"rule,omitempty"`
	MostRecentInstance *CodeScanningInstanceOutput `json:"most_recent_instance,omitempty"`
	DismissedReason    *string                     `json:"dismissed_reason,omitempty"`
	DismissedComment   *string                     `json:"dismissed_comment,omitempty"`
}

type SecretScanningLocationOutput struct {
	Path        *string `json:"path,omitempty"`
	StartLine   *int    `json:"start_line,omitempty"`
	EndLine     *int    `json:"end_line,omitempty"`
	CommitSHA   *string `json:"commit_sha,omitempty"`
	StartColumn *int    `json:"start_column,omitempty"`
	EndColumn   *int    `json:"end_column,omitempty"`
}

type SecretScanningAlertOutput struct {
	Number                 *int                          `json:"number,omitempty"`
	State                  *string                       `json:"state,omitempty"`
	Resolution             *string                       `json:"resolution,omitempty"`
	SecretType             *string                       `json:"secret_type,omitempty"`
	SecretTypeDisplayName  *string                       `json:"secret_type_display_name,omitempty"`
	Secret                 *string                       `json:"secret,omitempty"` //nolint:gosec // G117: this scoped tool intentionally returns the matched secret.
	HTMLURL                *string                       `json:"html_url,omitempty"`
	FirstLocationDetected  *SecretScanningLocationOutput `json:"first_location_detected,omitempty"`
	Validity               *string                       `json:"validity,omitempty" jsonschema:"Credential validity reported by GitHub: active, inactive, or unknown."`
	ResolutionComment      *string                       `json:"resolution_comment,omitempty"`
	PushProtectionBypassed *bool                         `json:"push_protection_bypassed,omitempty" jsonschema:"Whether push protection was bypassed for this secret."`
	IsBase64Encoded        *bool                         `json:"is_base64_encoded,omitempty" jsonschema:"Whether the detected secret is base64 encoded."`
	HasMoreLocations       *bool                         `json:"has_more_locations,omitempty" jsonschema:"Whether additional secret locations exist beyond the first location."`
}

type SecurityPackageOutput struct {
	Ecosystem *string `json:"ecosystem,omitempty"`
	Name      *string `json:"name,omitempty"`
}

type SecurityVulnerabilityOutput struct {
	Package                *SecurityPackageOutput `json:"package,omitempty"`
	Severity               *string                `json:"severity,omitempty"`
	VulnerableVersionRange *string                `json:"vulnerable_version_range,omitempty"`
	FirstPatchedVersion    *string                `json:"first_patched_version,omitempty" jsonschema:"Earliest version fixing the affected package."`
}

type DependabotDependencyOutput struct {
	Package      *SecurityPackageOutput `json:"package,omitempty"`
	ManifestPath *string                `json:"manifest_path,omitempty"`
	Scope        *string                `json:"scope,omitempty"`
}

type DependabotAdvisoryOutput struct {
	GHSAID         *string `json:"ghsa_id,omitempty"`
	CVEID          *string `json:"cve_id,omitempty"`
	Summary        *string `json:"summary,omitempty"`
	Description    *string `json:"description,omitempty"`
	Severity       *string `json:"severity,omitempty"`
	Classification *string `json:"classification,omitempty" jsonschema:"GitHub advisory classification, such as malware or general."`
}

type DependabotAlertOutput struct {
	Number                *int                         `json:"number,omitempty"`
	State                 *string                      `json:"state,omitempty"`
	Dependency            *DependabotDependencyOutput  `json:"dependency,omitempty"`
	SecurityAdvisory      *DependabotAdvisoryOutput    `json:"security_advisory,omitempty"`
	SecurityVulnerability *SecurityVulnerabilityOutput `json:"security_vulnerability,omitempty"`
	HTMLURL               *string                      `json:"html_url,omitempty"`
	DismissedReason       *string                      `json:"dismissed_reason,omitempty"`
	DismissedComment      *string                      `json:"dismissed_comment,omitempty"`
}

type SecurityAdvisoryVulnerabilityOutput struct {
	Package                *SecurityPackageOutput `json:"package,omitempty"`
	Severity               *string                `json:"severity,omitempty"`
	VulnerableVersionRange *string                `json:"vulnerable_version_range,omitempty"`
	FirstPatchedVersion    *string                `json:"first_patched_version,omitempty" jsonschema:"Earliest version fixing the affected package."`
	PatchedVersions        *string                `json:"patched_versions,omitempty"`
	VulnerableFunctions    []string               `json:"vulnerable_functions,omitempty"`
}

type SecurityAdvisoryOutput struct {
	GHSAID          *string                                `json:"ghsa_id,omitempty"`
	CVEID           *string                                `json:"cve_id,omitempty"`
	Summary         *string                                `json:"summary,omitempty"`
	Description     *string                                `json:"description,omitempty"`
	Severity        *string                                `json:"severity,omitempty"`
	State           *string                                `json:"state,omitempty"`
	HTMLURL         *string                                `json:"html_url,omitempty"`
	CWEIDs          []string                               `json:"cwe_ids,omitempty"`
	Vulnerabilities []*SecurityAdvisoryVulnerabilityOutput `json:"vulnerabilities,omitempty"`
	PublishedAt     *time.Time                             `json:"published_at,omitempty" jsonschema:"Advisory publication time (RFC 3339)."`
	UpdatedAt       *time.Time                             `json:"updated_at,omitempty" jsonschema:"Last advisory update time (RFC 3339)."`
	WithdrawnAt     *time.Time                             `json:"withdrawn_at,omitempty" jsonschema:"Withdrawal time, when withdrawn (RFC 3339)."`
}

type GlobalSecurityAdvisoryOutput struct {
	SecurityAdvisoryOutput
	Type               *string `json:"type,omitempty"`
	SourceCodeLocation *string `json:"source_code_location,omitempty"`
}

type SecurityPageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	NextCursor      string `json:"nextCursor,omitempty"`
	PrevCursor      string `json:"prevCursor,omitempty"`
}

type DependabotAlertsOutput struct {
	Alerts   []*DependabotAlertOutput `json:"alerts"`
	PageInfo SecurityPageInfo         `json:"pageInfo"`
}

func securityPagination(page, perPage *int) PaginationParams {
	pagination := PaginationParams{Page: 1, PerPage: 30}
	if page != nil {
		pagination.Page = *page
	}
	if perPage != nil {
		pagination.PerPage = *perPage
	}
	return pagination
}

func securityCursorPagination(perPage *int, after string) CursorPaginationParams {
	pagination := CursorPaginationParams{PerPage: 30, After: after}
	if perPage != nil {
		pagination.PerPage = *perPage
	}
	return pagination
}

func normalizeSecurityIntegerArguments(fields ...string) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var arguments map[string]json.RawMessage
		if err := json.Unmarshal(raw, &arguments); err != nil {
			return nil, err
		}

		for _, field := range fields {
			value, exists := arguments[field]
			if !exists {
				continue
			}
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, fmt.Errorf("parameter %s is not a valid number", field)
			}
			number := string(value)
			if len(number) > 0 && number[0] == '"' {
				if err := json.Unmarshal(value, &number); err != nil {
					return nil, fmt.Errorf("parameter %s is not a valid number: %w", field, err)
				}
			}
			parsed, err := strconv.ParseFloat(number, 64)
			if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) ||
				parsed != math.Trunc(parsed) || parsed > float64(math.MaxInt) || parsed < float64(math.MinInt) {
				return nil, fmt.Errorf("parameter %s is not a valid number", field)
			}
			arguments[field], err = json.Marshal(int(parsed))
			if err != nil {
				return nil, err
			}
		}
		if arguments == nil {
			return raw, nil
		}
		return json.Marshal(arguments)
	}
}

func normalizeGlobalAdvisoryArguments(raw json.RawMessage) (json.RawMessage, error) {
	var arguments map[string]json.RawMessage
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return nil, err
	}
	if value, exists := arguments["cwes"]; exists && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		delete(arguments, "cwes")
		return json.Marshal(arguments)
	}
	return raw, nil
}

func mapSecurityOutputs[In, Out any](items []*In, project func(*In) *Out) []*Out {
	if items == nil {
		return nil
	}
	outputs := make([]*Out, len(items))
	for i, item := range items {
		outputs[i] = project(item)
	}
	return outputs
}

func codeScanningAlertOutput(alert *github.Alert) *CodeScanningAlertOutput {
	if alert == nil {
		return nil
	}
	output := &CodeScanningAlertOutput{
		Number:          alert.Number,
		State:           alert.State,
		HTMLURL:         alert.HTMLURL,
		DismissedReason: alert.DismissedReason, DismissedComment: alert.DismissedComment,
	}
	if alert.Rule != nil {
		output.Rule = &CodeScanningRuleOutput{
			ID: alert.Rule.ID, Severity: alert.Rule.Severity,
			Description: alert.Rule.Description, Name: alert.Rule.Name,
			SecuritySeverityLevel: alert.Rule.SecuritySeverityLevel, Help: alert.Rule.Help,
		}
	}
	if instance := alert.MostRecentInstance; instance != nil {
		output.MostRecentInstance = &CodeScanningInstanceOutput{
			Ref: instance.Ref, State: instance.State, CommitSHA: instance.CommitSHA,
			HTMLURL: instance.HTMLURL,
		}
		if instance.Message != nil {
			output.MostRecentInstance.Message = instance.Message.Text
		}
		if location := instance.Location; location != nil {
			output.MostRecentInstance.Location = &CodeScanningLocationOutput{
				Path: location.Path, StartLine: location.StartLine, EndLine: location.EndLine,
				StartColumn: location.StartColumn, EndColumn: location.EndColumn,
			}
		}
	}
	return output
}

func secretScanningAlertOutput(alert *github.SecretScanningAlert) *SecretScanningAlertOutput {
	if alert == nil {
		return nil
	}
	output := &SecretScanningAlertOutput{
		Number: alert.Number, State: alert.State, Resolution: alert.Resolution,
		SecretType: alert.SecretType, SecretTypeDisplayName: alert.SecretTypeDisplayName,
		Secret: alert.Secret, HTMLURL: alert.HTMLURL,
		Validity: alert.Validity, ResolutionComment: alert.ResolutionComment,
		PushProtectionBypassed: alert.PushProtectionBypassed,
		IsBase64Encoded:        alert.IsBase64Encoded, HasMoreLocations: alert.HasMoreLocations,
	}
	if location := alert.FirstLocationDetected; location != nil {
		output.FirstLocationDetected = &SecretScanningLocationOutput{
			Path: location.Path, StartLine: location.Startline, EndLine: location.EndLine,
			CommitSHA: location.CommitSHA, StartColumn: location.StartColumn, EndColumn: location.EndColumn,
		}
	}
	return output
}

func securityPackageOutput(pkg *github.VulnerabilityPackage) *SecurityPackageOutput {
	if pkg == nil {
		return nil
	}
	return &SecurityPackageOutput{Ecosystem: pkg.Ecosystem, Name: pkg.Name}
}

func securityVulnerabilityOutput(vulnerability *github.AdvisoryVulnerability) *SecurityVulnerabilityOutput {
	if vulnerability == nil {
		return nil
	}
	output := &SecurityVulnerabilityOutput{
		Package: securityPackageOutput(vulnerability.Package), Severity: vulnerability.Severity,
		VulnerableVersionRange: vulnerability.VulnerableVersionRange,
	}
	if patched := vulnerability.FirstPatchedVersion; patched != nil {
		output.FirstPatchedVersion = patched.Identifier
	}
	return output
}

func dependabotAlertOutput(alert *github.DependabotAlert) *DependabotAlertOutput {
	if alert == nil {
		return nil
	}
	output := &DependabotAlertOutput{
		Number: alert.Number, State: alert.State, HTMLURL: alert.HTMLURL,
		DismissedReason: alert.DismissedReason, DismissedComment: alert.DismissedComment,
	}
	if dependency := alert.Dependency; dependency != nil {
		output.Dependency = &DependabotDependencyOutput{
			Package:      securityPackageOutput(dependency.Package),
			ManifestPath: dependency.ManifestPath, Scope: dependency.Scope,
		}
	}
	if advisory := alert.SecurityAdvisory; advisory != nil {
		output.SecurityAdvisory = &DependabotAdvisoryOutput{
			GHSAID: advisory.GHSAID, CVEID: advisory.CVEID, Summary: advisory.Summary,
			Description: advisory.Description, Severity: advisory.Severity,
			Classification: advisory.Classification,
		}
	}
	output.SecurityVulnerability = securityVulnerabilityOutput(alert.SecurityVulnerability)
	return output
}

func securityAdvisoryOutput(advisory *github.SecurityAdvisory) *SecurityAdvisoryOutput {
	if advisory == nil {
		return nil
	}
	output := &SecurityAdvisoryOutput{
		GHSAID: advisory.GHSAID, CVEID: advisory.CVEID, Summary: advisory.Summary,
		Description: advisory.Description, Severity: advisory.Severity, State: advisory.State,
		HTMLURL: advisory.HTMLURL, CWEIDs: advisory.CWEIDs,
		PublishedAt: githubTimestampTime(advisory.PublishedAt),
		UpdatedAt:   githubTimestampTime(advisory.UpdatedAt),
		WithdrawnAt: githubTimestampTime(advisory.WithdrawnAt),
	}
	if advisory.Vulnerabilities != nil {
		output.Vulnerabilities = mapSecurityOutputs(advisory.Vulnerabilities, securityAdvisoryVulnerabilityOutput)
	}
	return output
}

func securityAdvisoryVulnerabilityOutput(vulnerability *github.AdvisoryVulnerability) *SecurityAdvisoryVulnerabilityOutput {
	if vulnerability == nil {
		return nil
	}
	projected := securityVulnerabilityOutput(vulnerability)
	return &SecurityAdvisoryVulnerabilityOutput{
		Package: projected.Package, Severity: projected.Severity,
		VulnerableVersionRange: projected.VulnerableVersionRange,
		FirstPatchedVersion:    projected.FirstPatchedVersion,
		PatchedVersions:        vulnerability.PatchedVersions,
		VulnerableFunctions:    vulnerability.VulnerableFunctions,
	}
}

func globalSecurityAdvisoryOutput(advisory *github.GlobalSecurityAdvisory) *GlobalSecurityAdvisoryOutput {
	if advisory == nil {
		return nil
	}
	output := &GlobalSecurityAdvisoryOutput{
		SecurityAdvisoryOutput: *securityAdvisoryOutput(&advisory.SecurityAdvisory),
		Type:                   advisory.Type, SourceCodeLocation: advisory.SourceCodeLocation,
	}
	if len(output.CWEIDs) == 0 {
		for _, cwe := range advisory.CWEs {
			if cwe != nil && cwe.CWEID != nil {
				output.CWEIDs = append(output.CWEIDs, *cwe.CWEID)
			}
		}
	}
	if advisory.Vulnerabilities != nil {
		output.Vulnerabilities = mapSecurityOutputs(advisory.Vulnerabilities, globalAdvisoryVulnerabilityOutput)
	}
	return output
}

func globalAdvisoryVulnerabilityOutput(vulnerability *github.GlobalSecurityVulnerability) *SecurityAdvisoryVulnerabilityOutput {
	if vulnerability == nil {
		return nil
	}
	return &SecurityAdvisoryVulnerabilityOutput{
		Package:                securityPackageOutput(vulnerability.Package),
		FirstPatchedVersion:    vulnerability.FirstPatchedVersion,
		VulnerableVersionRange: vulnerability.VulnerableVersionRange,
		VulnerableFunctions:    vulnerability.VulnerableFunctions,
	}
}

func githubTimestampTime(timestamp *github.Timestamp) *time.Time {
	if timestamp == nil {
		return nil
	}
	return &timestamp.Time
}
