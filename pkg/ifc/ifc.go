// Package ifc provides Information Flow Control labels for annotating MCP tool outputs.
// The actual IFC enforcement engine lives in a separate service; this package only
// defines the label schema used for annotations.
package ifc

type Integrity string

const (
	IntegrityTrusted   Integrity = "trusted"
	IntegrityUntrusted Integrity = "untrusted"
)

type Confidentiality string

const (
	ConfidentialityPublic  Confidentiality = "public"
	ConfidentialityPrivate Confidentiality = "private"
)

type SecurityLabel struct {
	Integrity       Integrity       `json:"integrity"`
	Confidentiality Confidentiality `json:"confidentiality"`
}

// PublicTrusted returns a label for trusted, publicly readable data.
func PublicTrusted() SecurityLabel {
	return SecurityLabel{
		Integrity:       IntegrityTrusted,
		Confidentiality: ConfidentialityPublic,
	}
}

// PublicUntrusted returns a label for untrusted, publicly readable data.
func PublicUntrusted() SecurityLabel {
	return SecurityLabel{
		Integrity:       IntegrityUntrusted,
		Confidentiality: ConfidentialityPublic,
	}
}

// PrivateTrusted returns a label for trusted data restricted to the readers
// of the originating repository. The reader set is opaque on the wire (a
// single "private" marker); the client engine resolves the concrete readers
// from the GitHub API on demand at egress decision time.
func PrivateTrusted() SecurityLabel {
	return SecurityLabel{
		Integrity:       IntegrityTrusted,
		Confidentiality: ConfidentialityPrivate,
	}
}

// PrivateUntrusted returns a label for untrusted data restricted to the
// readers of the originating repository. See PrivateTrusted for the reader
// resolution model.
func PrivateUntrusted() SecurityLabel {
	return SecurityLabel{
		Integrity:       IntegrityUntrusted,
		Confidentiality: ConfidentialityPrivate,
	}
}

func LabelGetMe() SecurityLabel {
	return PublicTrusted()
}

// LabelListIssues returns the IFC label for a list_issues result.
// Public repositories are universally readable; private repositories are
// restricted to their collaborators (resolved client-side from the marker).
// Issue contents are attacker-controllable, so integrity is always untrusted.
func LabelListIssues(isPrivate bool) SecurityLabel {
	if isPrivate {
		return PrivateUntrusted()
	}
	return PublicUntrusted()
}

// LabelGetFileContents returns the IFC label for a get_file_contents result.
// Public repository file contents may be authored by anyone via pull requests
// and are therefore untrusted. In private repositories only collaborators can
// land changes, so contents are treated as trusted.
func LabelGetFileContents(isPrivate bool) SecurityLabel {
	if isPrivate {
		return PrivateTrusted()
	}
	return PublicUntrusted()
}

// LabelSearchIssues returns the IFC label for a multi-repository search
// result, joining per-repository labels across all matched repositories.
// Used by both search_issues and search_repositories.
//
// Integrity is always untrusted because results expose user-authored content.
//
// Confidentiality follows the IFC meet (greatest lower bound): if any matched
// repository is private the joined label is private; otherwise public. The
// reader set is opaque (the "private" marker); the client engine resolves
// concrete readers on demand at egress decision time.
//
// An empty result set is treated as public-untrusted (no repository data is
// leaked).
func LabelSearchIssues(repoVisibilities []bool) SecurityLabel {
	for _, isPrivate := range repoVisibilities {
		if isPrivate {
			return PrivateUntrusted()
		}
	}
	return PublicUntrusted()
}
