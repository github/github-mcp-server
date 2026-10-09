package github

import (
	"encoding/json"

	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
)

type SearchAccountsInput struct {
	Query   string `json:"query"`
	Sort    string `json:"sort,omitempty"`
	Order   string `json:"order,omitempty"`
	Page    *int   `json:"page,omitempty"`
	PerPage *int   `json:"perPage,omitempty"`
}

type SearchRepositoriesInput struct {
	SearchAccountsInput
	MinimalOutput *bool `json:"minimal_output,omitempty"`
}

type SearchCommitsInput = SearchAccountsInput

// SearchRepositoriesOutput is the structured repository search result. Minimal
// output reuses MinimalRepository; full output adds a compact set of
// repository details instead of the complete REST API object.
type SearchRepositoriesOutput struct {
	TotalCount        int                `json:"total_count"`
	IncompleteResults bool               `json:"incomplete_results"`
	Items             []SearchRepository `json:"items"`
}

type SearchRepository struct {
	MinimalRepository
	Homepage       string                       `json:"homepage,omitempty" jsonschema:"Repository homepage URL"`
	Visibility     string                       `json:"visibility,omitempty" jsonschema:"Repository visibility"`
	PushedAt       string                       `json:"pushed_at,omitempty" jsonschema:"Last push time as an ISO 8601 UTC timestamp"`
	WatchersCount  *int                         `json:"watchers_count,omitempty" jsonschema:"Number of watchers"`
	Size           *int                         `json:"size,omitempty" jsonschema:"Repository size in kilobytes"`
	Disabled       *bool                        `json:"disabled,omitempty" jsonschema:"Whether the repository is disabled"`
	IsTemplate     *bool                        `json:"is_template,omitempty" jsonschema:"Whether the repository is a template"`
	HasIssues      *bool                        `json:"has_issues,omitempty"`
	HasProjects    *bool                        `json:"has_projects,omitempty"`
	HasWiki        *bool                        `json:"has_wiki,omitempty"`
	HasDiscussions *bool                        `json:"has_discussions,omitempty"`
	License        *SearchRepositoryLicense     `json:"license,omitempty" jsonschema:"Repository license"`
	Permissions    *SearchRepositoryPermissions `json:"permissions,omitempty" jsonschema:"Authenticated user's repository permissions"`
}

type SearchRepositoryLicense struct {
	Key    string `json:"key,omitempty"`
	Name   string `json:"name,omitempty"`
	SPDXID string `json:"spdx_id,omitempty" jsonschema:"SPDX license identifier"`
}

type SearchRepositoryPermissions struct {
	Admin    *bool `json:"admin,omitempty"`
	Maintain *bool `json:"maintain,omitempty"`
	Push     *bool `json:"push,omitempty"`
	Triage   *bool `json:"triage,omitempty"`
	Pull     *bool `json:"pull,omitempty"`
}

var searchRepositoryVisibilityEnum = []any{"public", "private", "internal"}

func searchRepositoriesOutputSchema() *jsonschema.Schema {
	schema, err := jsonschema.For[SearchRepositoriesOutput](nil)
	if err != nil {
		panic(err)
	}
	schema.Properties["items"].Items.Properties["visibility"].Enum = searchRepositoryVisibilityEnum
	return schema
}

func minimalSearchRepository(repo *github.Repository) MinimalRepository {
	minimalRepo := MinimalRepository{
		ID:            repo.GetID(),
		Name:          repo.GetName(),
		FullName:      repo.GetFullName(),
		Description:   repo.GetDescription(),
		HTMLURL:       repo.GetHTMLURL(),
		Language:      repo.GetLanguage(),
		Stars:         repo.GetStargazersCount(),
		Forks:         repo.GetForksCount(),
		OpenIssues:    repo.GetOpenIssuesCount(),
		Private:       repo.GetPrivate(),
		Fork:          repo.GetFork(),
		Archived:      repo.GetArchived(),
		DefaultBranch: repo.GetDefaultBranch(),
	}
	if repo.UpdatedAt != nil {
		minimalRepo.UpdatedAt = repo.UpdatedAt.Format("2006-01-02T15:04:05Z")
	}
	if repo.CreatedAt != nil {
		minimalRepo.CreatedAt = repo.CreatedAt.Format("2006-01-02T15:04:05Z")
	}
	if repo.Topics != nil {
		minimalRepo.Topics = repo.Topics
	}
	return minimalRepo
}

func fullSearchRepository(repo *github.Repository) SearchRepository {
	out := SearchRepository{
		MinimalRepository: minimalSearchRepository(repo),
		Homepage:          repo.GetHomepage(),
		Visibility:        repo.GetVisibility(),
		WatchersCount:     repo.WatchersCount,
		Size:              repo.Size,
		Disabled:          repo.Disabled,
		IsTemplate:        repo.IsTemplate,
		HasIssues:         repo.HasIssues,
		HasProjects:       repo.HasProjects,
		HasWiki:           repo.HasWiki,
		HasDiscussions:    repo.HasDiscussions,
	}
	if repo.PushedAt != nil {
		out.PushedAt = repo.PushedAt.Format("2006-01-02T15:04:05Z")
	}
	if license := repo.License; license != nil {
		out.License = &SearchRepositoryLicense{Key: license.GetKey(), Name: license.GetName(), SPDXID: license.GetSPDXID()}
	}
	if permissions := repo.Permissions; permissions != nil {
		out.Permissions = &SearchRepositoryPermissions{
			Admin: permissions.Admin, Maintain: permissions.Maintain, Push: permissions.Push,
			Triage: permissions.Triage, Pull: permissions.Pull,
		}
	}
	return out
}

func normalizeSearchArguments(minimalOutput bool) func(json.RawMessage) (json.RawMessage, error) {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		if _, err := RequiredParam[string](args, "query"); err != nil {
			return nil, err
		}
		for _, field := range []string{"sort", "order"} {
			if _, err := OptionalParam[string](args, field); err != nil {
				return nil, err
			}
		}
		if _, err := OptionalPaginationParams(args); err != nil {
			return nil, err
		}
		if minimalOutput {
			if _, err := OptionalBoolParamWithDefault(args, "minimal_output", true); err != nil {
				return nil, err
			}
		}
		return normalizeRepositoryArguments(raw)
	}
}
