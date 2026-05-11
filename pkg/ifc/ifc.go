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

func LabelGetMe() SecurityLabel {
	return SecurityLabel{
		Integrity:       IntegrityTrusted,
		Confidentiality: []Confidentiality{ConfidentialityPublic},
	}
}
