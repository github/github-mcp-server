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
	ConfidentialityPublic Confidentiality = "public"
)

type SecurityLabel struct {
	Integrity       Integrity         `json:"integrity"`
	Confidentiality []Confidentiality `json:"confidentiality"`
}

// PublicTrusted returns a label for trusted, publicly readable data.
func PublicTrusted() SecurityLabel {
	return SecurityLabel{
		Integrity:       IntegrityTrusted,
		Confidentiality: []Confidentiality{ConfidentialityPublic},
	}
}

// PublicUntrusted returns a label for untrusted, publicly readable data.
func PublicUntrusted() SecurityLabel {
	return SecurityLabel{
		Integrity:       IntegrityUntrusted,
		Confidentiality: []Confidentiality{ConfidentialityPublic},
	}
}

// PrivateTrusted returns a label for trusted data restricted to the given readers.
func PrivateTrusted(readers []string) SecurityLabel {
	return SecurityLabel{
		Integrity:       IntegrityTrusted,
		Confidentiality: toConfidentiality(readers),
	}
}

// PrivateUntrusted returns a label for untrusted data restricted to the given readers.
func PrivateUntrusted(readers []string) SecurityLabel {
	return SecurityLabel{
		Integrity:       IntegrityUntrusted,
		Confidentiality: toConfidentiality(readers),
	}
}

func toConfidentiality(readers []string) []Confidentiality {
	out := make([]Confidentiality, len(readers))
	for i, r := range readers {
		out[i] = Confidentiality(r)
	}
	return out
}

func LabelGetMe() SecurityLabel {
	return PublicTrusted()
}

// LabelListIssues returns the IFC label for a list_issues result.
// Public repositories are universally readable; private repositories are
// restricted to the provided reader set (typically repository collaborators).
// Issue contents are attacker-controllable, so integrity is always untrusted.
func LabelListIssues(isPrivate bool, readers []string) SecurityLabel {
	if isPrivate {
		return PrivateUntrusted(readers)
	}
	return PublicUntrusted()
}

// LabelGetFileContents returns the IFC label for a get_file_contents result.
// Public repository file contents may be authored by anyone via pull requests
// and are therefore untrusted. In private repositories only collaborators can
// land changes, so contents are treated as trusted.
func LabelGetFileContents(isPrivate bool, readers []string) SecurityLabel {
	if isPrivate {
		return PrivateTrusted(readers)
	}
	return PublicUntrusted()
}
