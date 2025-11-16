package github

import "github.com/google/go-github/v79/github"

// MinimalUser is the output type for user and organization search results.
type MinimalUser struct {
	Login      string       `json:"login"`
	ID         int64        `json:"id,omitempty"`
	ProfileURL string       `json:"profile_url,omitempty"`
	AvatarURL  string       `json:"avatar_url,omitempty"`
	Details    *UserDetails `json:"details,omitempty"` // Optional field for additional user details
}

// MinimalSearchUsersResult is the trimmed output type for user search results.
type MinimalSearchUsersResult struct {
	TotalCount        int           `json:"total_count"`
	IncompleteResults bool          `json:"incomplete_results"`
	Items             []MinimalUser `json:"items"`
}

// MinimalRepository is the trimmed output type for repository objects to reduce verbosity.
type MinimalRepository struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	FullName      string   `json:"full_name"`
	Description   string   `json:"description,omitempty"`
	HTMLURL       string   `json:"html_url"`
	Language      string   `json:"language,omitempty"`
	Stars         int      `json:"stargazers_count"`
	Forks         int      `json:"forks_count"`
	OpenIssues    int      `json:"open_issues_count"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
	Topics        []string `json:"topics,omitempty"`
	Private       bool     `json:"private"`
	Fork          bool     `json:"fork"`
	Archived      bool     `json:"archived"`
	DefaultBranch string   `json:"default_branch,omitempty"`
}

// MinimalSearchRepositoriesResult is the trimmed output type for repository search results.
type MinimalSearchRepositoriesResult struct {
	TotalCount        int                 `json:"total_count"`
	IncompleteResults bool                `json:"incomplete_results"`
	Items             []MinimalRepository `json:"items"`
}

// MinimalCommitAuthor represents commit author information.
type MinimalCommitAuthor struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Date  string `json:"date,omitempty"`
}

// MinimalCommitInfo represents core commit information.
type MinimalCommitInfo struct {
	Message   string               `json:"message"`
	Author    *MinimalCommitAuthor `json:"author,omitempty"`
	Committer *MinimalCommitAuthor `json:"committer,omitempty"`
}

// MinimalCommitStats represents commit statistics.
type MinimalCommitStats struct {
	Additions int `json:"additions,omitempty"`
	Deletions int `json:"deletions,omitempty"`
	Total     int `json:"total,omitempty"`
}

// MinimalCommitFile represents a file changed in a commit.
type MinimalCommitFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status,omitempty"`
	Additions int    `json:"additions,omitempty"`
	Deletions int    `json:"deletions,omitempty"`
	Changes   int    `json:"changes,omitempty"`
}

// MinimalCommit is the trimmed output type for commit objects.
type MinimalCommit struct {
	SHA       string              `json:"sha"`
	HTMLURL   string              `json:"html_url"`
	Commit    *MinimalCommitInfo  `json:"commit,omitempty"`
	Author    *MinimalUser        `json:"author,omitempty"`
	Committer *MinimalUser        `json:"committer,omitempty"`
	Stats     *MinimalCommitStats `json:"stats,omitempty"`
	Files     []MinimalCommitFile `json:"files,omitempty"`
}

// MinimalRelease is the trimmed output type for release objects.
type MinimalRelease struct {
	ID          int64        `json:"id"`
	TagName     string       `json:"tag_name"`
	Name        string       `json:"name,omitempty"`
	Body        string       `json:"body,omitempty"`
	HTMLURL     string       `json:"html_url"`
	PublishedAt string       `json:"published_at,omitempty"`
	Prerelease  bool         `json:"prerelease"`
	Draft       bool         `json:"draft"`
	Author      *MinimalUser `json:"author,omitempty"`
}

// MinimalBranch is the trimmed output type for branch objects.
type MinimalBranch struct {
	Name      string `json:"name"`
	SHA       string `json:"sha"`
	Protected bool   `json:"protected"`
}

// MinimalResponse represents a minimal response for all CRUD operations.
// Success is implicit in the HTTP response status, and all other information
// can be derived from the URL or fetched separately if needed.
type MinimalResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type MinimalProject struct {
	ID               *int64            `json:"id,omitempty"`
	NodeID           *string           `json:"node_id,omitempty"`
	Owner            *MinimalUser      `json:"owner,omitempty"`
	Creator          *MinimalUser      `json:"creator,omitempty"`
	Title            *string           `json:"title,omitempty"`
	Description      *string           `json:"description,omitempty"`
	Public           *bool             `json:"public,omitempty"`
	ClosedAt         *github.Timestamp `json:"closed_at,omitempty"`
	CreatedAt        *github.Timestamp `json:"created_at,omitempty"`
	UpdatedAt        *github.Timestamp `json:"updated_at,omitempty"`
	DeletedAt        *github.Timestamp `json:"deleted_at,omitempty"`
	Number           *int              `json:"number,omitempty"`
	ShortDescription *string           `json:"short_description,omitempty"`
	DeletedBy        *MinimalUser      `json:"deleted_by,omitempty"`
}

// Helper functions

func convertToMinimalProject(fullProject *github.ProjectV2) *MinimalProject {
	if fullProject == nil {
		return nil
	}

	return &MinimalProject{
		ID:               github.Ptr(fullProject.GetID()),
		NodeID:           github.Ptr(fullProject.GetNodeID()),
		Owner:            convertToMinimalUser(fullProject.GetOwner()),
		Creator:          convertToMinimalUser(fullProject.GetCreator()),
		Title:            github.Ptr(fullProject.GetTitle()),
		Description:      github.Ptr(fullProject.GetDescription()),
		Public:           github.Ptr(fullProject.GetPublic()),
		ClosedAt:         github.Ptr(fullProject.GetClosedAt()),
		CreatedAt:        github.Ptr(fullProject.GetCreatedAt()),
		UpdatedAt:        github.Ptr(fullProject.GetUpdatedAt()),
		DeletedAt:        github.Ptr(fullProject.GetDeletedAt()),
		Number:           github.Ptr(fullProject.GetNumber()),
		ShortDescription: github.Ptr(fullProject.GetShortDescription()),
		DeletedBy:        convertToMinimalUser(fullProject.GetDeletedBy()),
	}
}

func convertToMinimalUser(user *github.User) *MinimalUser {
	if user == nil {
		return nil
	}

	return &MinimalUser{
		Login:      user.GetLogin(),
		ID:         user.GetID(),
		ProfileURL: user.GetHTMLURL(),
		AvatarURL:  user.GetAvatarURL(),
	}
}

// convertToMinimalCommit converts a GitHub API RepositoryCommit to MinimalCommit
func convertToMinimalCommit(commit *github.RepositoryCommit, includeDiffs bool) MinimalCommit {
	minimalCommit := MinimalCommit{
		SHA:     commit.GetSHA(),
		HTMLURL: commit.GetHTMLURL(),
	}

	if commit.Commit != nil {
		minimalCommit.Commit = &MinimalCommitInfo{
			Message: commit.Commit.GetMessage(),
		}

		if commit.Commit.Author != nil {
			minimalCommit.Commit.Author = &MinimalCommitAuthor{
				Name:  commit.Commit.Author.GetName(),
				Email: commit.Commit.Author.GetEmail(),
			}
			if commit.Commit.Author.Date != nil {
				minimalCommit.Commit.Author.Date = commit.Commit.Author.Date.Format("2006-01-02T15:04:05Z")
			}
		}

		if commit.Commit.Committer != nil {
			minimalCommit.Commit.Committer = &MinimalCommitAuthor{
				Name:  commit.Commit.Committer.GetName(),
				Email: commit.Commit.Committer.GetEmail(),
			}
			if commit.Commit.Committer.Date != nil {
				minimalCommit.Commit.Committer.Date = commit.Commit.Committer.Date.Format("2006-01-02T15:04:05Z")
			}
		}
	}

	if commit.Author != nil {
		minimalCommit.Author = &MinimalUser{
			Login:      commit.Author.GetLogin(),
			ID:         commit.Author.GetID(),
			ProfileURL: commit.Author.GetHTMLURL(),
			AvatarURL:  commit.Author.GetAvatarURL(),
		}
	}

	if commit.Committer != nil {
		minimalCommit.Committer = &MinimalUser{
			Login:      commit.Committer.GetLogin(),
			ID:         commit.Committer.GetID(),
			ProfileURL: commit.Committer.GetHTMLURL(),
			AvatarURL:  commit.Committer.GetAvatarURL(),
		}
	}

	// Only include stats and files if includeDiffs is true
	if includeDiffs {
		if commit.Stats != nil {
			minimalCommit.Stats = &MinimalCommitStats{
				Additions: commit.Stats.GetAdditions(),
				Deletions: commit.Stats.GetDeletions(),
				Total:     commit.Stats.GetTotal(),
			}
		}

		if len(commit.Files) > 0 {
			minimalCommit.Files = make([]MinimalCommitFile, 0, len(commit.Files))
			for _, file := range commit.Files {
				minimalFile := MinimalCommitFile{
					Filename:  file.GetFilename(),
					Status:    file.GetStatus(),
					Additions: file.GetAdditions(),
					Deletions: file.GetDeletions(),
					Changes:   file.GetChanges(),
				}
				minimalCommit.Files = append(minimalCommit.Files, minimalFile)
			}
		}
	}

	return minimalCommit
}

// convertToMinimalBranch converts a GitHub API Branch to MinimalBranch
func convertToMinimalBranch(branch *github.Branch) MinimalBranch {
	return MinimalBranch{
		Name:      branch.GetName(),
		SHA:       branch.GetCommit().GetSHA(),
		Protected: branch.GetProtected(),
	}
}
