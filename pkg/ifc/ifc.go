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

// LabelSearchIssues returns the IFC label for a search_issues result, joining
// per-repository labels across all matched repositories.
//
// Integrity is always untrusted because issue contents are user-authored.
//
// Confidentiality follows the IFC join (least upper bound):
//   - If any matched repository is public, the joined readers are ["public"]
//     (the agent can already observe public content as soon as one match is
//     public, so the public side dominates).
//   - Otherwise the joined readers are the intersection of the per-repository
//     reader sets (a reader must have access to every matched private
//     repository).
//   - If no repositories matched (empty result set), the label is treated as
//     public-untrusted because no repository data is leaked.
//
// repoVisibilities[i] reports whether the i-th matched repository is private;
// readerSets[i] is that repository's reader set (only consulted for private
// repos). The two slices must have the same length.
func LabelSearchIssues(repoVisibilities []bool, readerSets [][]string) SecurityLabel {
	if len(repoVisibilities) == 0 {
		return PublicUntrusted()
	}
	for _, isPrivate := range repoVisibilities {
		if !isPrivate {
			return PublicUntrusted()
		}
	}
	return PrivateUntrusted(intersectReaders(readerSets))
}

// intersectReaders returns the readers present in every set, preserving the
// order from the first set. Empty input yields nil.
func intersectReaders(sets [][]string) []string {
	if len(sets) == 0 {
		return nil
	}
	counts := make(map[string]int, len(sets[0]))
	for _, login := range sets[0] {
		if _, seen := counts[login]; seen {
			continue
		}
		counts[login] = 1
	}
	for _, set := range sets[1:] {
		seen := make(map[string]struct{}, len(set))
		for _, login := range set {
			if _, dup := seen[login]; dup {
				continue
			}
			seen[login] = struct{}{}
			if _, ok := counts[login]; ok {
				counts[login]++
			}
		}
	}
	out := make([]string, 0, len(counts))
	for _, login := range sets[0] {
		if counts[login] == len(sets) {
			out = append(out, login)
			delete(counts, login)
		}
	}
	return out
}
