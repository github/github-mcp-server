package github

import (
	"encoding/json"
	"time"

	"github.com/google/go-github/v89/github"
)

// SecurityUserOutput is the compact user reference used by security findings.
type SecurityUserOutput struct {
	Login   *string `json:"login,omitempty"`
	ID      *int64  `json:"id,omitempty"`
	HTMLURL *string `json:"html_url,omitempty"`
}

// SecurityMessageOutput contains the rendered forms of a security finding message.
type SecurityMessageOutput struct {
	Text     *string `json:"text,omitempty"`
	Markdown *string `json:"markdown,omitempty"`
}

// SecurityLocationOutput identifies the source range associated with a finding.
type SecurityLocationOutput struct {
	Path        *string `json:"path,omitempty"`
	StartLine   *int    `json:"start_line,omitempty"`
	StartColumn *int    `json:"start_column,omitempty"`
	EndLine     *int    `json:"end_line,omitempty"`
	EndColumn   *int    `json:"end_column,omitempty"`
}

// CodeQualityRuleOutput describes the rule that produced a code quality finding.
type CodeQualityRuleOutput struct {
	ID          *string `json:"id,omitempty"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Help        *string `json:"help,omitempty"`
	Severity    *string `json:"severity,omitempty"`
	Category    *string `json:"category,omitempty"`
}

// CodeQualityFindingOutput is the typed output for get_code_quality_finding.
type CodeQualityFindingOutput struct {
	Number    *int                    `json:"number,omitempty"`
	State     *string                 `json:"state,omitempty"`
	URL       *string                 `json:"url,omitempty"`
	Rule      *CodeQualityRuleOutput  `json:"rule,omitempty"`
	Location  *SecurityLocationOutput `json:"location,omitempty"`
	Message   *SecurityMessageOutput  `json:"message,omitempty"`
	CreatedAt *string                 `json:"created_at,omitempty"`

	rawFields map[string]json.RawMessage
}

func (finding *CodeQualityFindingOutput) UnmarshalJSON(data []byte) error {
	type wire CodeQualityFindingOutput
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawFields); err != nil {
		return err
	}

	*finding = CodeQualityFindingOutput(decoded)
	finding.rawFields = rawFields
	return nil
}

func (finding CodeQualityFindingOutput) MarshalJSON() ([]byte, error) {
	type wire CodeQualityFindingOutput
	if finding.rawFields == nil {
		return json.Marshal(wire(finding))
	}

	projected := make(map[string]json.RawMessage, len(finding.rawFields))
	for name, raw := range finding.rawFields {
		switch name {
		case "number", "state", "url", "created_at":
			projected[name] = raw
		case "rule":
			value, err := projectRawObjectFields(raw, "id", "title", "description", "help", "severity", "category")
			if err != nil {
				return nil, err
			}
			projected[name] = value
		case "location":
			value, err := projectRawObjectFields(raw, "path", "start_line", "start_column", "end_line", "end_column")
			if err != nil {
				return nil, err
			}
			projected[name] = value
		case "message":
			value, err := projectRawObjectFields(raw, "text", "markdown")
			if err != nil {
				return nil, err
			}
			projected[name] = value
		}
	}
	return json.Marshal(projected)
}

func projectRawObjectFields(raw json.RawMessage, fields ...string) (json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return raw, nil
	}

	projected := make(map[string]json.RawMessage, len(fields))
	for _, field := range fields {
		if value, ok := object[field]; ok {
			projected[field] = value
		}
	}
	return json.Marshal(projected)
}

// CodeScanningRuleOutput describes the rule that produced a code scanning alert.
type CodeScanningRuleOutput struct {
	ID                    *string   `json:"id,omitempty"`
	Severity              *string   `json:"severity,omitempty"`
	Description           *string   `json:"description,omitempty"`
	Name                  *string   `json:"name,omitempty"`
	SecuritySeverityLevel *string   `json:"security_severity_level,omitempty"`
	FullDescription       *string   `json:"full_description,omitempty"`
	Tags                  *[]string `json:"tags,omitempty"`
	Help                  *string   `json:"help,omitempty"`
}

// CodeScanningToolOutput identifies the analysis tool that produced an alert.
type CodeScanningToolOutput struct {
	Name    *string `json:"name,omitempty"`
	GUID    *string `json:"guid,omitempty"`
	Version *string `json:"version,omitempty"`
}

// CodeScanningInstanceOutput describes one occurrence of a code scanning alert.
type CodeScanningInstanceOutput struct {
	Ref             *string                 `json:"ref,omitempty"`
	AnalysisKey     *string                 `json:"analysis_key,omitempty"`
	Category        *string                 `json:"category,omitempty"`
	Environment     *string                 `json:"environment,omitempty"`
	State           *string                 `json:"state,omitempty"`
	CommitSHA       *string                 `json:"commit_sha,omitempty"`
	Message         *SecurityMessageOutput  `json:"message,omitempty"`
	Location        *SecurityLocationOutput `json:"location,omitempty"`
	HTMLURL         *string                 `json:"html_url,omitempty"`
	Classifications *[]string               `json:"classifications,omitempty"`
}

// CodeScanningAlertOutput is the compact output for code scanning alert tools.
type CodeScanningAlertOutput struct {
	Number             *int                           `json:"number,omitempty"`
	RuleID             *string                        `json:"rule_id,omitempty"`
	RuleSeverity       *string                        `json:"rule_severity,omitempty"`
	RuleDescription    *string                        `json:"rule_description,omitempty"`
	Rule               *CodeScanningRuleOutput        `json:"rule,omitempty"`
	Tool               *CodeScanningToolOutput        `json:"tool,omitempty"`
	CreatedAt          *string                        `json:"created_at,omitempty"`
	UpdatedAt          *string                        `json:"updated_at,omitempty"`
	FixedAt            *string                        `json:"fixed_at,omitempty"`
	State              *string                        `json:"state,omitempty"`
	ClosedBy           *SecurityUserOutput            `json:"closed_by,omitempty"`
	ClosedAt           *string                        `json:"closed_at,omitempty"`
	URL                *string                        `json:"url,omitempty"`
	HTMLURL            *string                        `json:"html_url,omitempty"`
	MostRecentInstance *CodeScanningInstanceOutput    `json:"most_recent_instance,omitempty"`
	Instances          *[]*CodeScanningInstanceOutput `json:"instances,omitempty"`
	DismissedBy        *SecurityUserOutput            `json:"dismissed_by,omitempty"`
	DismissedAt        *string                        `json:"dismissed_at,omitempty"`
	DismissedReason    *string                        `json:"dismissed_reason,omitempty"`
	DismissedComment   *string                        `json:"dismissed_comment,omitempty"`
	InstancesURL       *string                        `json:"instances_url,omitempty"`
}

// VulnerabilityPackageOutput identifies an affected package.
type VulnerabilityPackageOutput struct {
	Ecosystem *string `json:"ecosystem,omitempty"`
	Name      *string `json:"name,omitempty"`
}

// DependabotDependencyOutput identifies the dependency that triggered an alert.
type DependabotDependencyOutput struct {
	Package      *VulnerabilityPackageOutput `json:"package,omitempty"`
	ManifestPath *string                     `json:"manifest_path,omitempty"`
	Scope        *string                     `json:"scope,omitempty"`
}

// SecurityAdvisoryCVSSOutput contains one CVSS score and vector.
type SecurityAdvisoryCVSSOutput struct {
	Score        *float64 `json:"score,omitempty"`
	VectorString *string  `json:"vector_string,omitempty"`
}

// SecurityAdvisoryCVSSSeveritiesOutput contains versioned CVSS scores.
type SecurityAdvisoryCVSSSeveritiesOutput struct {
	CVSSV3 *SecurityAdvisoryCVSSOutput `json:"cvss_v3,omitempty"`
	CVSSV4 *SecurityAdvisoryCVSSOutput `json:"cvss_v4,omitempty"`
}

// SecurityAdvisoryEPSSOutput contains exploit prediction scores.
type SecurityAdvisoryEPSSOutput struct {
	Percentage float64 `json:"percentage"`
	Percentile float64 `json:"percentile"`
}

// SecurityAdvisoryCWEOutput identifies a Common Weakness Enumeration.
type SecurityAdvisoryCWEOutput struct {
	CWEID *string `json:"cwe_id,omitempty"`
	Name  *string `json:"name,omitempty"`
}

// SecurityAdvisoryIdentifierOutput contains an advisory identifier.
type SecurityAdvisoryIdentifierOutput struct {
	Value *string `json:"value,omitempty"`
	Type  *string `json:"type,omitempty"`
}

// SecurityAdvisoryReferenceOutput contains an advisory reference URL.
type SecurityAdvisoryReferenceOutput struct {
	URL *string `json:"url,omitempty"`
}

// FirstPatchedVersionOutput identifies the first version containing a fix.
type FirstPatchedVersionOutput struct {
	Identifier *string `json:"identifier,omitempty"`
}

// SecurityVulnerabilityOutput describes one affected package and version range.
type SecurityVulnerabilityOutput struct {
	Package                *VulnerabilityPackageOutput `json:"package,omitempty"`
	Severity               *string                     `json:"severity,omitempty"`
	VulnerableVersionRange *string                     `json:"vulnerable_version_range,omitempty"`
	FirstPatchedVersion    *FirstPatchedVersionOutput  `json:"first_patched_version,omitempty"`
	PatchedVersions        *string                     `json:"patched_versions,omitempty"`
	VulnerableFunctions    *[]string                   `json:"vulnerable_functions,omitempty"`
}

// DependabotSecurityAdvisoryOutput contains the advisory attached to an alert.
type DependabotSecurityAdvisoryOutput struct {
	GHSAID          *string                               `json:"ghsa_id,omitempty"`
	CVEID           *string                               `json:"cve_id,omitempty"`
	Summary         *string                               `json:"summary,omitempty"`
	Description     *string                               `json:"description,omitempty"`
	Vulnerabilities *[]*SecurityVulnerabilityOutput       `json:"vulnerabilities,omitempty"`
	Severity        *string                               `json:"severity,omitempty"`
	Classification  *string                               `json:"classification,omitempty"`
	CVSS            *SecurityAdvisoryCVSSOutput           `json:"cvss,omitempty"`
	CVSSSeverities  *SecurityAdvisoryCVSSSeveritiesOutput `json:"cvss_severities,omitempty"`
	CWEs            *[]*SecurityAdvisoryCWEOutput         `json:"cwes,omitempty"`
	EPSS            *SecurityAdvisoryEPSSOutput           `json:"epss,omitempty"`
	Identifiers     *[]*SecurityAdvisoryIdentifierOutput  `json:"identifiers,omitempty"`
	References      *[]*SecurityAdvisoryReferenceOutput   `json:"references,omitempty"`
	PublishedAt     *string                               `json:"published_at,omitempty"`
	UpdatedAt       *string                               `json:"updated_at,omitempty"`
	WithdrawnAt     *string                               `json:"withdrawn_at,omitempty"`
}

// DependabotAlertOutput is the compact output for Dependabot alert tools.
type DependabotAlertOutput struct {
	Number                *int                              `json:"number,omitempty"`
	State                 *string                           `json:"state,omitempty"`
	Dependency            *DependabotDependencyOutput       `json:"dependency,omitempty"`
	SecurityAdvisory      *DependabotSecurityAdvisoryOutput `json:"security_advisory,omitempty"`
	SecurityVulnerability *SecurityVulnerabilityOutput      `json:"security_vulnerability,omitempty"`
	URL                   *string                           `json:"url,omitempty"`
	HTMLURL               *string                           `json:"html_url,omitempty"`
	CreatedAt             *string                           `json:"created_at,omitempty"`
	UpdatedAt             *string                           `json:"updated_at,omitempty"`
	DismissedAt           *string                           `json:"dismissed_at,omitempty"`
	DismissedBy           *SecurityUserOutput               `json:"dismissed_by,omitempty"`
	DismissedReason       *string                           `json:"dismissed_reason,omitempty"`
	DismissedComment      *string                           `json:"dismissed_comment,omitempty"`
	FixedAt               *string                           `json:"fixed_at,omitempty"`
	AutoDismissedAt       *string                           `json:"auto_dismissed_at,omitempty"`
}

// CursorPageInfoOutput describes cursor pagination returned by Dependabot.
type CursorPageInfoOutput struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	NextCursor      string `json:"nextCursor,omitempty"`
	PrevCursor      string `json:"prevCursor,omitempty"`
}

// DependabotAlertsOutput is the typed paginated output for list_dependabot_alerts.
type DependabotAlertsOutput struct {
	Alerts   []*DependabotAlertOutput `json:"alerts"`
	PageInfo CursorPageInfoOutput     `json:"pageInfo"`
}

// SecretScanningLocationOutput identifies where a secret was first detected.
type SecretScanningLocationOutput struct {
	Path                  *string `json:"path,omitempty"`
	StartLine             *int    `json:"start_line,omitempty"`
	EndLine               *int    `json:"end_line,omitempty"`
	StartColumn           *int    `json:"start_column,omitempty"`
	EndColumn             *int    `json:"end_column,omitempty"`
	BlobSHA               *string `json:"blob_sha,omitempty"`
	BlobURL               *string `json:"blob_url,omitempty"`
	CommitSHA             *string `json:"commit_sha,omitempty"`
	CommitURL             *string `json:"commit_url,omitempty"`
	PullRequestCommentURL *string `json:"pull_request_comment_url,omitempty"`
}

// SecretScanningAlertOutput is the compact output for secret scanning alert tools.
type SecretScanningAlertOutput struct {
	Number                                     *int                          `json:"number,omitempty"`
	CreatedAt                                  *string                       `json:"created_at,omitempty"`
	URL                                        *string                       `json:"url,omitempty"`
	HTMLURL                                    *string                       `json:"html_url,omitempty"`
	LocationsURL                               *string                       `json:"locations_url,omitempty"`
	FirstLocationDetected                      *SecretScanningLocationOutput `json:"first_location_detected,omitempty"`
	HasMoreLocations                           *bool                         `json:"has_more_locations,omitempty"`
	State                                      *string                       `json:"state,omitempty"`
	Resolution                                 *string                       `json:"resolution,omitempty"`
	ResolvedAt                                 *string                       `json:"resolved_at,omitempty"`
	ResolvedBy                                 *SecurityUserOutput           `json:"resolved_by,omitempty"`
	SecretType                                 *string                       `json:"secret_type,omitempty"`
	SecretTypeDisplayName                      *string                       `json:"secret_type_display_name,omitempty"`
	Secret                                     *string                       `json:"secret,omitempty"`
	UpdatedAt                                  *string                       `json:"updated_at,omitempty"`
	IsBase64Encoded                            *bool                         `json:"is_base64_encoded,omitempty"`
	MultiRepo                                  *bool                         `json:"multi_repo,omitempty"`
	PubliclyLeaked                             *bool                         `json:"publicly_leaked,omitempty"`
	PushProtectionBypassed                     *bool                         `json:"push_protection_bypassed,omitempty"`
	PushProtectionBypassedBy                   *SecurityUserOutput           `json:"push_protection_bypassed_by,omitempty"`
	PushProtectionBypassedAt                   *string                       `json:"push_protection_bypassed_at,omitempty"`
	ResolutionComment                          *string                       `json:"resolution_comment,omitempty"`
	PushProtectionBypassRequestComment         *string                       `json:"push_protection_bypass_request_comment,omitempty"`
	PushProtectionBypassRequestHTMLURL         *string                       `json:"push_protection_bypass_request_html_url,omitempty"`
	PushProtectionBypassRequestReviewer        *SecurityUserOutput           `json:"push_protection_bypass_request_reviewer,omitempty"`
	PushProtectionBypassRequestReviewerComment *string                       `json:"push_protection_bypass_request_reviewer_comment,omitempty"`
	Validity                                   *string                       `json:"validity,omitempty"`
}

// SecurityAdvisorySubmissionOutput describes a private report submission decision.
type SecurityAdvisorySubmissionOutput struct {
	Accepted *bool `json:"accepted,omitempty"`
}

// SecurityAdvisoryCreditOutput identifies a credited researcher.
type SecurityAdvisoryCreditOutput struct {
	Login *string `json:"login,omitempty"`
	Type  *string `json:"type,omitempty"`
}

// SecurityAdvisoryDetailedCreditOutput includes credit state and user details.
type SecurityAdvisoryDetailedCreditOutput struct {
	User  *SecurityUserOutput `json:"user,omitempty"`
	Type  *string             `json:"type,omitempty"`
	State *string             `json:"state,omitempty"`
}

// RepositorySecurityAdvisoryOutput is the compact output for repository advisory lists.
type RepositorySecurityAdvisoryOutput struct {
	CVSS            *SecurityAdvisoryCVSSOutput              `json:"cvss,omitempty"`
	CVSSSeverities  *SecurityAdvisoryCVSSSeveritiesOutput    `json:"cvss_severities,omitempty"`
	CWEs            *[]*SecurityAdvisoryCWEOutput            `json:"cwes,omitempty"`
	GHSAID          *string                                  `json:"ghsa_id,omitempty"`
	Summary         *string                                  `json:"summary,omitempty"`
	Description     *string                                  `json:"description,omitempty"`
	Severity        *string                                  `json:"severity,omitempty"`
	Identifiers     *[]*SecurityAdvisoryIdentifierOutput     `json:"identifiers,omitempty"`
	References      *[]*SecurityAdvisoryReferenceOutput      `json:"references,omitempty"`
	PublishedAt     *string                                  `json:"published_at,omitempty"`
	UpdatedAt       *string                                  `json:"updated_at,omitempty"`
	WithdrawnAt     *string                                  `json:"withdrawn_at,omitempty"`
	Vulnerabilities *[]*SecurityVulnerabilityOutput          `json:"vulnerabilities,omitempty"`
	CVEID           *string                                  `json:"cve_id,omitempty"`
	URL             *string                                  `json:"url,omitempty"`
	HTMLURL         *string                                  `json:"html_url,omitempty"`
	Author          *SecurityUserOutput                      `json:"author,omitempty"`
	Publisher       *SecurityUserOutput                      `json:"publisher,omitempty"`
	State           *string                                  `json:"state,omitempty"`
	CreatedAt       *string                                  `json:"created_at,omitempty"`
	ClosedAt        *string                                  `json:"closed_at,omitempty"`
	Submission      *SecurityAdvisorySubmissionOutput        `json:"submission,omitempty"`
	CWEIDs          *[]string                                `json:"cwe_ids,omitempty"`
	Credits         *[]*SecurityAdvisoryCreditOutput         `json:"credits,omitempty"`
	CreditsDetailed *[]*SecurityAdvisoryDetailedCreditOutput `json:"credits_detailed,omitempty"`
}

func convertCodeScanningAlertOutput(alert *github.Alert) *CodeScanningAlertOutput {
	if alert == nil {
		return nil
	}
	return &CodeScanningAlertOutput{
		Number:             alert.Number,
		RuleID:             alert.RuleID,
		RuleSeverity:       alert.RuleSeverity,
		RuleDescription:    alert.RuleDescription,
		Rule:               convertCodeScanningRuleOutput(alert.Rule),
		Tool:               convertCodeScanningToolOutput(alert.Tool),
		CreatedAt:          securityTimestampOutput(alert.CreatedAt),
		UpdatedAt:          securityTimestampOutput(alert.UpdatedAt),
		FixedAt:            securityTimestampOutput(alert.FixedAt),
		State:              alert.State,
		ClosedBy:           convertSecurityUserOutput(alert.ClosedBy),
		ClosedAt:           securityTimestampOutput(alert.ClosedAt),
		URL:                alert.URL,
		HTMLURL:            alert.HTMLURL,
		MostRecentInstance: convertCodeScanningInstanceOutput(alert.MostRecentInstance),
		Instances:          convertPointerSliceOutput(alert.Instances, convertCodeScanningInstanceOutput),
		DismissedBy:        convertSecurityUserOutput(alert.DismissedBy),
		DismissedAt:        securityTimestampOutput(alert.DismissedAt),
		DismissedReason:    alert.DismissedReason,
		DismissedComment:   alert.DismissedComment,
		InstancesURL:       alert.InstancesURL,
	}
}

func convertCodeScanningRuleOutput(rule *github.Rule) *CodeScanningRuleOutput {
	if rule == nil {
		return nil
	}
	return &CodeScanningRuleOutput{
		ID:                    rule.ID,
		Severity:              rule.Severity,
		Description:           rule.Description,
		Name:                  rule.Name,
		SecuritySeverityLevel: rule.SecuritySeverityLevel,
		FullDescription:       rule.FullDescription,
		Tags:                  stringSliceOutput(rule.Tags),
		Help:                  rule.Help,
	}
}

func convertCodeScanningToolOutput(tool *github.Tool) *CodeScanningToolOutput {
	if tool == nil {
		return nil
	}
	return &CodeScanningToolOutput{
		Name:    tool.Name,
		GUID:    tool.GUID,
		Version: tool.Version,
	}
}

func convertCodeScanningInstanceOutput(instance *github.MostRecentInstance) *CodeScanningInstanceOutput {
	if instance == nil {
		return nil
	}
	return &CodeScanningInstanceOutput{
		Ref:             instance.Ref,
		AnalysisKey:     instance.AnalysisKey,
		Category:        instance.Category,
		Environment:     instance.Environment,
		State:           instance.State,
		CommitSHA:       instance.CommitSHA,
		Message:         convertSecurityMessageOutput(instance.Message),
		Location:        convertSecurityLocationOutput(instance.Location),
		HTMLURL:         instance.HTMLURL,
		Classifications: stringSliceOutput(instance.Classifications),
	}
}

func convertSecurityMessageOutput(message *github.Message) *SecurityMessageOutput {
	if message == nil {
		return nil
	}
	return &SecurityMessageOutput{Text: message.Text}
}

func convertSecurityLocationOutput(location *github.Location) *SecurityLocationOutput {
	if location == nil {
		return nil
	}
	return &SecurityLocationOutput{
		Path:        location.Path,
		StartLine:   location.StartLine,
		StartColumn: location.StartColumn,
		EndLine:     location.EndLine,
		EndColumn:   location.EndColumn,
	}
}

func convertDependabotAlertOutput(alert *github.DependabotAlert) *DependabotAlertOutput {
	if alert == nil {
		return nil
	}
	return &DependabotAlertOutput{
		Number:                alert.Number,
		State:                 alert.State,
		Dependency:            convertDependabotDependencyOutput(alert.Dependency),
		SecurityAdvisory:      convertDependabotSecurityAdvisoryOutput(alert.SecurityAdvisory),
		SecurityVulnerability: convertSecurityVulnerabilityOutput(alert.SecurityVulnerability),
		URL:                   alert.URL,
		HTMLURL:               alert.HTMLURL,
		CreatedAt:             securityTimestampOutput(alert.CreatedAt),
		UpdatedAt:             securityTimestampOutput(alert.UpdatedAt),
		DismissedAt:           securityTimestampOutput(alert.DismissedAt),
		DismissedBy:           convertSecurityUserOutput(alert.DismissedBy),
		DismissedReason:       alert.DismissedReason,
		DismissedComment:      alert.DismissedComment,
		FixedAt:               securityTimestampOutput(alert.FixedAt),
		AutoDismissedAt:       securityTimestampOutput(alert.AutoDismissedAt),
	}
}

func convertDependabotDependencyOutput(dependency *github.Dependency) *DependabotDependencyOutput {
	if dependency == nil {
		return nil
	}
	return &DependabotDependencyOutput{
		Package:      convertVulnerabilityPackageOutput(dependency.Package),
		ManifestPath: dependency.ManifestPath,
		Scope:        dependency.Scope,
	}
}

func convertDependabotSecurityAdvisoryOutput(advisory *github.DependabotSecurityAdvisory) *DependabotSecurityAdvisoryOutput {
	if advisory == nil {
		return nil
	}
	return &DependabotSecurityAdvisoryOutput{
		GHSAID:          advisory.GHSAID,
		CVEID:           advisory.CVEID,
		Summary:         advisory.Summary,
		Description:     advisory.Description,
		Vulnerabilities: convertPointerSliceOutput(advisory.Vulnerabilities, convertSecurityVulnerabilityOutput),
		Severity:        advisory.Severity,
		Classification:  advisory.Classification,
		CVSS:            convertSecurityAdvisoryCVSSOutput(advisory.CVSS),
		CVSSSeverities:  convertSecurityAdvisoryCVSSSeveritiesOutput(advisory.CVSSSeverities),
		CWEs:            convertPointerSliceOutput(advisory.CWEs, convertSecurityAdvisoryCWEOutput),
		EPSS:            convertSecurityAdvisoryEPSSOutput(advisory.EPSS),
		Identifiers:     convertPointerSliceOutput(advisory.Identifiers, convertSecurityAdvisoryIdentifierOutput),
		References:      convertPointerSliceOutput(advisory.References, convertSecurityAdvisoryReferenceOutput),
		PublishedAt:     securityTimestampOutput(advisory.PublishedAt),
		UpdatedAt:       securityTimestampOutput(advisory.UpdatedAt),
		WithdrawnAt:     securityTimestampOutput(advisory.WithdrawnAt),
	}
}

func convertSecretScanningAlertOutput(alert *github.SecretScanningAlert) *SecretScanningAlertOutput {
	if alert == nil {
		return nil
	}
	return &SecretScanningAlertOutput{
		Number:                              alert.Number,
		CreatedAt:                           securityTimestampOutput(alert.CreatedAt),
		URL:                                 alert.URL,
		HTMLURL:                             alert.HTMLURL,
		LocationsURL:                        alert.LocationsURL,
		FirstLocationDetected:               convertSecretScanningLocationOutput(alert.FirstLocationDetected),
		HasMoreLocations:                    alert.HasMoreLocations,
		State:                               alert.State,
		Resolution:                          alert.Resolution,
		ResolvedAt:                          securityTimestampOutput(alert.ResolvedAt),
		ResolvedBy:                          convertSecurityUserOutput(alert.ResolvedBy),
		SecretType:                          alert.SecretType,
		SecretTypeDisplayName:               alert.SecretTypeDisplayName,
		Secret:                              alert.Secret,
		UpdatedAt:                           securityTimestampOutput(alert.UpdatedAt),
		IsBase64Encoded:                     alert.IsBase64Encoded,
		MultiRepo:                           alert.MultiRepo,
		PubliclyLeaked:                      alert.PubliclyLeaked,
		PushProtectionBypassed:              alert.PushProtectionBypassed,
		PushProtectionBypassedBy:            convertSecurityUserOutput(alert.PushProtectionBypassedBy),
		PushProtectionBypassedAt:            securityTimestampOutput(alert.PushProtectionBypassedAt),
		ResolutionComment:                   alert.ResolutionComment,
		PushProtectionBypassRequestComment:  alert.PushProtectionBypassRequestComment,
		PushProtectionBypassRequestHTMLURL:  alert.PushProtectionBypassRequestHTMLURL,
		PushProtectionBypassRequestReviewer: convertSecurityUserOutput(alert.PushProtectionBypassRequestReviewer),
		PushProtectionBypassRequestReviewerComment: alert.PushProtectionBypassRequestReviewerComment,
		Validity: alert.Validity,
	}
}

func convertSecretScanningLocationOutput(location *github.SecretScanningAlertLocationDetails) *SecretScanningLocationOutput {
	if location == nil {
		return nil
	}
	return &SecretScanningLocationOutput{
		Path:                  location.Path,
		StartLine:             location.Startline,
		EndLine:               location.EndLine,
		StartColumn:           location.StartColumn,
		EndColumn:             location.EndColumn,
		BlobSHA:               location.BlobSHA,
		BlobURL:               location.BlobURL,
		CommitSHA:             location.CommitSHA,
		CommitURL:             location.CommitURL,
		PullRequestCommentURL: location.PullRequestCommentURL,
	}
}

func convertRepositorySecurityAdvisoryOutput(advisory *github.SecurityAdvisory) *RepositorySecurityAdvisoryOutput {
	if advisory == nil {
		return nil
	}
	return &RepositorySecurityAdvisoryOutput{
		CVSS:            convertSecurityAdvisoryCVSSOutput(advisory.CVSS),
		CVSSSeverities:  convertSecurityAdvisoryCVSSSeveritiesOutput(advisory.CVSSSeverities),
		CWEs:            convertPointerSliceOutput(advisory.CWEs, convertSecurityAdvisoryCWEOutput),
		GHSAID:          advisory.GHSAID,
		Summary:         advisory.Summary,
		Description:     advisory.Description,
		Severity:        advisory.Severity,
		Identifiers:     convertPointerSliceOutput(advisory.Identifiers, convertSecurityAdvisoryIdentifierOutput),
		References:      convertPointerSliceOutput(advisory.References, convertSecurityAdvisoryReferenceOutput),
		PublishedAt:     securityTimestampOutput(advisory.PublishedAt),
		UpdatedAt:       securityTimestampOutput(advisory.UpdatedAt),
		WithdrawnAt:     securityTimestampOutput(advisory.WithdrawnAt),
		Vulnerabilities: convertPointerSliceOutput(advisory.Vulnerabilities, convertSecurityVulnerabilityOutput),
		CVEID:           advisory.CVEID,
		URL:             advisory.URL,
		HTMLURL:         advisory.HTMLURL,
		Author:          convertSecurityUserOutput(advisory.Author),
		Publisher:       convertSecurityUserOutput(advisory.Publisher),
		State:           advisory.State,
		CreatedAt:       securityTimestampOutput(advisory.CreatedAt),
		ClosedAt:        securityTimestampOutput(advisory.ClosedAt),
		Submission:      convertSecurityAdvisorySubmissionOutput(advisory.Submission),
		CWEIDs:          stringSliceOutput(advisory.CWEIDs),
		Credits:         convertPointerSliceOutput(advisory.Credits, convertSecurityAdvisoryCreditOutput),
		CreditsDetailed: convertPointerSliceOutput(advisory.CreditsDetailed, convertSecurityAdvisoryDetailedCreditOutput),
	}
}

func convertSecurityVulnerabilityOutput(vulnerability *github.AdvisoryVulnerability) *SecurityVulnerabilityOutput {
	if vulnerability == nil {
		return nil
	}
	return &SecurityVulnerabilityOutput{
		Package:                convertVulnerabilityPackageOutput(vulnerability.Package),
		Severity:               vulnerability.Severity,
		VulnerableVersionRange: vulnerability.VulnerableVersionRange,
		FirstPatchedVersion:    convertFirstPatchedVersionOutput(vulnerability.FirstPatchedVersion),
		PatchedVersions:        vulnerability.PatchedVersions,
		VulnerableFunctions:    stringSliceOutput(vulnerability.VulnerableFunctions),
	}
}

func convertFirstPatchedVersionOutput(version *github.FirstPatchedVersion) *FirstPatchedVersionOutput {
	if version == nil {
		return nil
	}
	return &FirstPatchedVersionOutput{Identifier: version.Identifier}
}

func convertVulnerabilityPackageOutput(pkg *github.VulnerabilityPackage) *VulnerabilityPackageOutput {
	if pkg == nil {
		return nil
	}
	return &VulnerabilityPackageOutput{
		Ecosystem: pkg.Ecosystem,
		Name:      pkg.Name,
	}
}

func convertSecurityAdvisoryCVSSOutput(cvss *github.AdvisoryCVSS) *SecurityAdvisoryCVSSOutput {
	if cvss == nil {
		return nil
	}
	return &SecurityAdvisoryCVSSOutput{
		Score:        cvss.Score,
		VectorString: cvss.VectorString,
	}
}

func convertSecurityAdvisoryCVSSSeveritiesOutput(severities *github.AdvisoryCVSSSeverities) *SecurityAdvisoryCVSSSeveritiesOutput {
	if severities == nil {
		return nil
	}
	return &SecurityAdvisoryCVSSSeveritiesOutput{
		CVSSV3: convertSecurityAdvisoryCVSSOutput(severities.CVSSV3),
		CVSSV4: convertSecurityAdvisoryCVSSOutput(severities.CVSSV4),
	}
}

func convertSecurityAdvisoryEPSSOutput(epss *github.AdvisoryEPSS) *SecurityAdvisoryEPSSOutput {
	if epss == nil {
		return nil
	}
	return &SecurityAdvisoryEPSSOutput{
		Percentage: epss.Percentage,
		Percentile: epss.Percentile,
	}
}

func convertSecurityAdvisoryCWEOutput(cwe *github.AdvisoryCWEs) *SecurityAdvisoryCWEOutput {
	if cwe == nil {
		return nil
	}
	return &SecurityAdvisoryCWEOutput{
		CWEID: cwe.CWEID,
		Name:  cwe.Name,
	}
}

func convertSecurityAdvisoryIdentifierOutput(identifier *github.AdvisoryIdentifier) *SecurityAdvisoryIdentifierOutput {
	if identifier == nil {
		return nil
	}
	return &SecurityAdvisoryIdentifierOutput{
		Value: identifier.Value,
		Type:  identifier.Type,
	}
}

func convertSecurityAdvisoryReferenceOutput(reference *github.AdvisoryReference) *SecurityAdvisoryReferenceOutput {
	if reference == nil {
		return nil
	}
	return &SecurityAdvisoryReferenceOutput{URL: reference.URL}
}

func convertSecurityAdvisorySubmissionOutput(submission *github.SecurityAdvisorySubmission) *SecurityAdvisorySubmissionOutput {
	if submission == nil {
		return nil
	}
	return &SecurityAdvisorySubmissionOutput{Accepted: submission.Accepted}
}

func convertSecurityAdvisoryCreditOutput(credit *github.RepoAdvisoryCredit) *SecurityAdvisoryCreditOutput {
	if credit == nil {
		return nil
	}
	return &SecurityAdvisoryCreditOutput{
		Login: credit.Login,
		Type:  credit.Type,
	}
}

func convertSecurityAdvisoryDetailedCreditOutput(credit *github.RepoAdvisoryCreditDetailed) *SecurityAdvisoryDetailedCreditOutput {
	if credit == nil {
		return nil
	}
	return &SecurityAdvisoryDetailedCreditOutput{
		User:  convertSecurityUserOutput(credit.User),
		Type:  credit.Type,
		State: credit.State,
	}
}

func convertSecurityUserOutput(user *github.User) *SecurityUserOutput {
	if user == nil {
		return nil
	}
	return &SecurityUserOutput{
		Login:   user.Login,
		ID:      user.ID,
		HTMLURL: user.HTMLURL,
	}
}

func securityTimestampOutput(timestamp *github.Timestamp) *string {
	if timestamp == nil {
		return nil
	}
	value := timestamp.Time.Format(time.RFC3339Nano)
	return &value
}

func stringSliceOutput(values []string) *[]string {
	if values == nil {
		return nil
	}
	result := make([]string, len(values))
	copy(result, values)
	return &result
}

func convertPointerSliceOutput[In, Out any](values []*In, convert func(*In) *Out) *[]*Out {
	if values == nil {
		return nil
	}
	result := make([]*Out, len(values))
	for i, value := range values {
		result[i] = convert(value)
	}
	return &result
}

func convertPointerListOutput[In, Out any](values []*In, convert func(*In) *Out) []*Out {
	if values == nil {
		return nil
	}
	result := make([]*Out, len(values))
	for i, value := range values {
		result[i] = convert(value)
	}
	return result
}
