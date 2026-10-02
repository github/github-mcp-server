package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/github/github-mcp-server/pkg/inventory"
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
	URL       *string                    `json:"url,omitempty"`
	Rule      *CodeQualityRuleOutput     `json:"rule,omitempty"`
	Location  *CodeQualityLocationOutput `json:"location,omitempty"`
	Message   *CodeQualityMessageOutput  `json:"message,omitempty"`
	CreatedAt *time.Time                 `json:"created_at,omitempty"`
}

type CodeScanningRuleOutput struct {
	ID          *string `json:"id,omitempty"`
	Severity    *string `json:"severity,omitempty"`
	Description *string `json:"description,omitempty"`
	Name        *string `json:"name,omitempty"`
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
	URL                *string                     `json:"url,omitempty"`
	HTMLURL            *string                     `json:"html_url,omitempty"`
	Rule               *CodeScanningRuleOutput     `json:"rule,omitempty"`
	MostRecentInstance *CodeScanningInstanceOutput `json:"most_recent_instance,omitempty"`
}

type SecretScanningLocationOutput struct {
	Path      *string `json:"path,omitempty"`
	StartLine *int    `json:"start_line,omitempty"`
	EndLine   *int    `json:"end_line,omitempty"`
	CommitSHA *string `json:"commit_sha,omitempty"`
	CommitURL *string `json:"commit_url,omitempty"`
}

type SecretScanningAlertOutput struct {
	Number                *int                          `json:"number,omitempty"`
	State                 *string                       `json:"state,omitempty"`
	Resolution            *string                       `json:"resolution,omitempty"`
	SecretType            *string                       `json:"secret_type,omitempty"`
	SecretTypeDisplayName *string                       `json:"secret_type_display_name,omitempty"`
	Secret                *string                       `json:"secret,omitempty"` //nolint:gosec // G117: this scoped tool intentionally returns the matched secret.
	URL                   *string                       `json:"url,omitempty"`
	HTMLURL               *string                       `json:"html_url,omitempty"`
	FirstLocationDetected *SecretScanningLocationOutput `json:"first_location_detected,omitempty"`
}

type SecurityPackageOutput struct {
	Ecosystem *string `json:"ecosystem,omitempty"`
	Name      *string `json:"name,omitempty"`
}

type SecurityVulnerabilityOutput struct {
	Package                *SecurityPackageOutput `json:"package,omitempty"`
	Severity               *string                `json:"severity,omitempty"`
	VulnerableVersionRange *string                `json:"vulnerable_version_range,omitempty"`
	FirstPatchedVersion    *string                `json:"first_patched_version,omitempty"`
}

type DependabotDependencyOutput struct {
	Package      *SecurityPackageOutput `json:"package,omitempty"`
	ManifestPath *string                `json:"manifest_path,omitempty"`
	Scope        *string                `json:"scope,omitempty"`
}

type DependabotAdvisoryOutput struct {
	GHSAID          *string                        `json:"ghsa_id,omitempty"`
	CVEID           *string                        `json:"cve_id,omitempty"`
	Summary         *string                        `json:"summary,omitempty"`
	Description     *string                        `json:"description,omitempty"`
	Severity        *string                        `json:"severity,omitempty"`
	Vulnerabilities []*SecurityVulnerabilityOutput `json:"vulnerabilities,omitempty"`
}

type DependabotAlertOutput struct {
	Number                *int                         `json:"number,omitempty"`
	State                 *string                      `json:"state,omitempty"`
	Dependency            *DependabotDependencyOutput  `json:"dependency,omitempty"`
	SecurityAdvisory      *DependabotAdvisoryOutput    `json:"security_advisory,omitempty"`
	SecurityVulnerability *SecurityVulnerabilityOutput `json:"security_vulnerability,omitempty"`
	URL                   *string                      `json:"url,omitempty"`
	HTMLURL               *string                      `json:"html_url,omitempty"`
}

type SecurityAdvisoryVulnerabilityOutput struct {
	Package                *SecurityPackageOutput `json:"package,omitempty"`
	Severity               *string                `json:"severity,omitempty"`
	VulnerableVersionRange *string                `json:"vulnerable_version_range,omitempty"`
	FirstPatchedVersion    *string                `json:"first_patched_version,omitempty"`
}

type SecurityAdvisoryOutput struct {
	GHSAID          *string                                `json:"ghsa_id,omitempty"`
	CVEID           *string                                `json:"cve_id,omitempty"`
	Summary         *string                                `json:"summary,omitempty"`
	Description     *string                                `json:"description,omitempty"`
	Severity        *string                                `json:"severity,omitempty"`
	State           *string                                `json:"state,omitempty"`
	URL             *string                                `json:"url,omitempty"`
	HTMLURL         *string                                `json:"html_url,omitempty"`
	CWEIDs          []string                               `json:"cwe_ids,omitempty"`
	Vulnerabilities []*SecurityAdvisoryVulnerabilityOutput `json:"vulnerabilities,omitempty"`
	PublishedAt     *time.Time                             `json:"published_at,omitempty"`
	UpdatedAt       *time.Time                             `json:"updated_at,omitempty"`
	WithdrawnAt     *time.Time                             `json:"withdrawn_at,omitempty"`
}

type GlobalSecurityAdvisoryOutput struct {
	SecurityAdvisoryOutput
	ID                    *int64     `json:"id,omitempty"`
	Type                  *string    `json:"type,omitempty"`
	SourceCodeLocation    *string    `json:"source_code_location,omitempty"`
	RepositoryAdvisoryURL *string    `json:"repository_advisory_url,omitempty"`
	References            []string   `json:"references,omitempty"`
	GithubReviewedAt      *time.Time `json:"github_reviewed_at,omitempty"`
	NVDPublishedAt        *time.Time `json:"nvd_published_at,omitempty"`
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
		Number:  alert.Number,
		State:   alert.State,
		URL:     alert.URL,
		HTMLURL: alert.HTMLURL,
	}
	if alert.Rule != nil {
		output.Rule = &CodeScanningRuleOutput{
			ID: alert.Rule.ID, Severity: alert.Rule.Severity,
			Description: alert.Rule.Description, Name: alert.Rule.Name,
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
		Secret: alert.Secret, URL: alert.URL, HTMLURL: alert.HTMLURL,
	}
	if location := alert.FirstLocationDetected; location != nil {
		output.FirstLocationDetected = &SecretScanningLocationOutput{
			Path: location.Path, StartLine: location.Startline, EndLine: location.EndLine,
			CommitSHA: location.CommitSHA, CommitURL: location.CommitURL,
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
		Number: alert.Number, State: alert.State, URL: alert.URL, HTMLURL: alert.HTMLURL,
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
		}
		if advisory.Vulnerabilities != nil {
			output.SecurityAdvisory.Vulnerabilities = mapSecurityOutputs(advisory.Vulnerabilities, securityVulnerabilityOutput)
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
		URL: advisory.URL, HTMLURL: advisory.HTMLURL, CWEIDs: advisory.CWEIDs,
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
	}
}

func globalSecurityAdvisoryOutput(advisory *github.GlobalSecurityAdvisory) *GlobalSecurityAdvisoryOutput {
	if advisory == nil {
		return nil
	}
	output := &GlobalSecurityAdvisoryOutput{
		SecurityAdvisoryOutput: *securityAdvisoryOutput(&advisory.SecurityAdvisory),
		ID:                     advisory.ID, Type: advisory.Type, SourceCodeLocation: advisory.SourceCodeLocation,
		RepositoryAdvisoryURL: advisory.RepositoryAdvisoryURL, References: advisory.References,
		GithubReviewedAt: githubTimestampTime(advisory.GithubReviewedAt),
		NVDPublishedAt:   githubTimestampTime(advisory.NVDPublishedAt),
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
	}
}

func githubTimestampTime(timestamp *github.Timestamp) *time.Time {
	if timestamp == nil {
		return nil
	}
	return &timestamp.Time
}
