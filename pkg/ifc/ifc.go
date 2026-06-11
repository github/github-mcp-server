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

// LabelGetMe returns the IFC label for the authenticated user's own profile
// (get_me).
//
// Integrity is trusted: this is GitHub-maintained data about the caller's own
// account, not attacker-authored content.
//
// Confidentiality is private. The result includes fields that are NOT part of
// the user's public profile — private_gists, total_private_repos, and
// owned_private_repos — which are visible only to the authenticated user. The
// result therefore must not be treated as world-readable.
func LabelGetMe() SecurityLabel {
	return PrivateTrusted()
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
//
// Why a single joined label rather than one label per item: a tool result is
// delivered as one opaque payload (a single content block) and the IFC engine
// makes one allow/deny decision per flow at egress. Once the items share a
// buffer in the agent's context they can be copied anywhere together, so the
// only sound bound for the whole result is the meet of every item's label.
// Per-item labels would only become load-bearing if the enforcement engine
// could partition a result and route individual items to different sinks;
// until then they would invite unsafe declassification of a "public" item that
// actually arrived alongside private data.
func LabelSearchIssues(repoVisibilities []bool) SecurityLabel {
	for _, isPrivate := range repoVisibilities {
		if isPrivate {
			return PrivateUntrusted()
		}
	}
	return PublicUntrusted()
}

// LabelRepoMetadata returns the IFC label for structural repository metadata
// that only collaborators with write access can define: labels, branches,
// tags, releases, issue types, issue field definitions, discussion
// categories, and the collaborator roster.
//
// Integrity is trusted because, unlike issue/PR/comment bodies, these
// artifacts cannot be authored by arbitrary outsiders — creating a branch,
// tag, release, or label requires push access, so the data reflects decisions
// made by the repository's trusted writers rather than attacker-controllable
// input.
//
// Confidentiality follows repository visibility: public repositories are
// universally readable; private repositories restrict the reader set (the
// opaque "private" marker, resolved client-side at egress time).
func LabelRepoMetadata(isPrivate bool) SecurityLabel {
	if isPrivate {
		return PrivateTrusted()
	}
	return PublicTrusted()
}

// LabelRelease returns the IFC label for repository releases (list_releases,
// get_latest_release, get_release_by_tag).
//
// Integrity is trusted: releases are published by collaborators with push
// access, not by arbitrary outsiders.
//
// Confidentiality is public only when the repository is public AND no returned
// release is a draft. Draft releases are visible only to users with push
// access — they are NOT world-readable even on a public repository — so a
// result containing one must be private. hasDraft reflects whether any release
// in the result is a draft; private repositories are always private regardless.
func LabelRelease(isPrivate bool, hasDraft bool) SecurityLabel {
	if isPrivate || hasDraft {
		return PrivateTrusted()
	}
	return PublicTrusted()
}

// LabelCollaboratorRoster returns the IFC label for a repository's collaborator
// list (list_repository_collaborators).
//
// Integrity is trusted: the roster is GitHub-maintained membership data, not
// attacker-authored content.
//
// Confidentiality is always private. Listing collaborators requires push
// access to the repository, so the roster is never world-readable — not even
// for public repositories. This mirrors LabelTeam: membership data is
// restricted regardless of the repository's own visibility.
func LabelCollaboratorRoster() SecurityLabel {
	return PrivateTrusted()
}

// LabelCommitContents returns the IFC label for committed repository content
// reachable from the default branch and its history: commits, commit diffs,
// and the repository file tree.
//
// It shares the reasoning of LabelGetFileContents. In public repositories any
// outsider can land content via a pull request, so the integrity of committed
// content is untrusted. In private repositories only collaborators can push,
// so committed content is trusted. Confidentiality follows repository
// visibility.
func LabelCommitContents(isPrivate bool) SecurityLabel {
	if isPrivate {
		return PrivateTrusted()
	}
	return PublicUntrusted()
}

// LabelActionsResult returns the IFC label for GitHub Actions resources:
// workflow definitions, runs, jobs, artifacts, and job logs.
//
// Integrity is untrusted. Workflow logs echo arbitrary text produced during a
// run — including output derived from pull-request branches, dependency
// downloads, and other attacker-influenceable sources — so log and artifact
// content must be treated as low integrity. Workflow definitions are
// themselves editable through pull requests in public repositories.
//
// Confidentiality follows repository visibility.
func LabelActionsResult(isPrivate bool) SecurityLabel {
	if isPrivate {
		return PrivateUntrusted()
	}
	return PublicUntrusted()
}

// LabelSecurityAlert returns the IFC label for security findings: code
// scanning alerts, secret scanning alerts, and Dependabot alerts.
//
// Integrity is untrusted because alert payloads embed attacker-influenceable
// material — the offending code snippet, the matched secret string, or a
// vulnerable dependency's advisory text — none of which the agent should treat
// as a trustworthy instruction source.
//
// Confidentiality is always private. Security alerts are access-restricted by
// GitHub regardless of repository visibility (only users with a security role
// can read them), so the reader set is narrow even for public repositories.
// Secret scanning results additionally surface the secret material itself.
func LabelSecurityAlert() SecurityLabel {
	return PrivateUntrusted()
}

// LabelGlobalSecurityAdvisory returns the IFC label for advisories served from
// the public GitHub Advisory Database (global advisories).
//
// The advisory database is world-readable, so confidentiality is public.
// Integrity is untrusted: advisory descriptions are externally authored prose
// and must not be treated as a trusted instruction source.
func LabelGlobalSecurityAdvisory() SecurityLabel {
	return PublicUntrusted()
}

// LabelRepositorySecurityAdvisory returns the IFC label for repository- or
// organization-scoped security advisories.
//
// Integrity is untrusted (externally authored advisory prose).
//
// Confidentiality is public only when the repository is public AND every
// advisory in the result is in the "published" state. Repository security
// advisories also exist in draft, triage, and closed states; those are visible
// only to maintainers and are NOT world-readable even on a public repository.
// Treating any non-published advisory as private (allPublished == false)
// prevents misclassifying an unpublished advisory from a public repo as
// public-readable. Private repositories are always private regardless of state.
func LabelRepositorySecurityAdvisory(isPrivate bool, allPublished bool) SecurityLabel {
	if isPrivate || !allPublished {
		return PrivateUntrusted()
	}
	return PublicUntrusted()
}

// LabelGist returns the IFC label for gist content.
//
// Integrity is untrusted: gist contents are arbitrary user-authored text.
// Confidentiality derives from the gist's own visibility rather than any
// repository — public gists are universally readable, while secret gists are
// restricted to those who hold the gist URL (modeled with the opaque "private"
// marker).
func LabelGist(isPublic bool) SecurityLabel {
	if isPublic {
		return PublicUntrusted()
	}
	return PrivateUntrusted()
}

// LabelGistList returns the IFC label for a list of gists belonging to a user,
// joining the per-gist confidentiality across the result set.
//
// Integrity is untrusted (user-authored content). Confidentiality follows the
// IFC meet: if any gist in the result is secret the joined label is private;
// otherwise public. An empty result is treated as public-untrusted.
//
// See LabelSearchIssues for why list results carry a single joined label
// rather than one label per item.
func LabelGistList(gistVisibilities []bool) SecurityLabel {
	for _, isPublic := range gistVisibilities {
		if !isPublic {
			return PrivateUntrusted()
		}
	}
	return PublicUntrusted()
}

// LabelProject returns the IFC label for a GitHub Project (Projects v2) and its
// items, status updates, and field definitions.
//
// Integrity is untrusted: project titles, item content, and status update
// bodies are user-authored free text. Confidentiality derives from the
// project's own public flag — public projects are universally readable, while
// private projects restrict the reader set.
func LabelProject(isPublic bool) SecurityLabel {
	if isPublic {
		return PublicUntrusted()
	}
	return PrivateUntrusted()
}

// LabelTeam returns the IFC label for organization team membership data
// (get_teams, get_team_members).
//
// Integrity is trusted: team membership is maintained by GitHub and cannot be
// forged by outside contributors, so it is not an attacker-controllable
// instruction source.
//
// Confidentiality is private. Organization team rosters and the teams a user
// belongs to are visible only to members of the organization, not to the
// public, so the reader set is restricted (the opaque "private" marker).
func LabelTeam() SecurityLabel {
	return PrivateTrusted()
}

// LabelNotificationDetails returns the IFC label for the subject of a single
// notification.
//
// Integrity is untrusted: a notification subject points at an issue, pull
// request, comment, or discussion whose content is user-authored and may carry
// attacker-controlled text. Confidentiality is private because notifications
// are delivered to a specific recipient and may reference private
// repositories; the result cannot be assumed to be publicly readable.
func LabelNotificationDetails() SecurityLabel {
	return PrivateUntrusted()
}
