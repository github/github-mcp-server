package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yosida95/uritemplate/v3"
)

var (
	repositoryResourceContentURITemplate       = uritemplate.MustNew("repo://{owner}/{repo}/contents{/path*}")
	repositoryResourceBranchContentURITemplate = uritemplate.MustNew("repo://{owner}/{repo}/refs/heads/{branch}/contents{/path*}")
	repositoryResourceCommitContentURITemplate = uritemplate.MustNew("repo://{owner}/{repo}/sha/{sha}/contents{/path*}")
	repositoryResourceTagContentURITemplate    = uritemplate.MustNew("repo://{owner}/{repo}/refs/tags/{tag}/contents{/path*}")
	repositoryResourcePrContentURITemplate     = uritemplate.MustNew("repo://{owner}/{repo}/refs/pull/{prNumber}/head/contents{/path*}")
)

// GetRepositoryResourceContent defines the resource template for getting repository content.
func GetRepositoryResourceContent(t translations.TranslationHelperFunc) inventory.ServerResourceTemplate {
	return inventory.NewServerResourceTemplate(
		ToolsetMetadataRepos,
		mcp.ResourceTemplate{
			Name:        "repository_content",
			URITemplate: repositoryResourceContentURITemplate.Raw(),
			Description: t("RESOURCE_REPOSITORY_CONTENT_DESCRIPTION", "Repository Content"),
			Icons:       octicons.Icons("repo"),
		},
		repositoryResourceContentsHandlerFunc(repositoryResourceContentURITemplate),
	)
}

// GetRepositoryResourceBranchContent defines the resource template for getting repository content for a branch.
func GetRepositoryResourceBranchContent(t translations.TranslationHelperFunc) inventory.ServerResourceTemplate {
	return inventory.NewServerResourceTemplate(
		ToolsetMetadataRepos,
		mcp.ResourceTemplate{
			Name:        "repository_content_branch",
			URITemplate: repositoryResourceBranchContentURITemplate.Raw(),
			Description: t("RESOURCE_REPOSITORY_CONTENT_BRANCH_DESCRIPTION", "Repository Content for specific branch"),
			Icons:       octicons.Icons("git-branch"),
		},
		repositoryResourceContentsHandlerFunc(repositoryResourceBranchContentURITemplate),
	)
}

// GetRepositoryResourceCommitContent defines the resource template for getting repository content for a commit.
func GetRepositoryResourceCommitContent(t translations.TranslationHelperFunc) inventory.ServerResourceTemplate {
	return inventory.NewServerResourceTemplate(
		ToolsetMetadataRepos,
		mcp.ResourceTemplate{
			Name:        "repository_content_commit",
			URITemplate: repositoryResourceCommitContentURITemplate.Raw(),
			Description: t("RESOURCE_REPOSITORY_CONTENT_COMMIT_DESCRIPTION", "Repository Content for specific commit"),
			Icons:       octicons.Icons("git-commit"),
		},
		repositoryResourceContentsHandlerFunc(repositoryResourceCommitContentURITemplate),
	)
}

// GetRepositoryResourceTagContent defines the resource template for getting repository content for a tag.
func GetRepositoryResourceTagContent(t translations.TranslationHelperFunc) inventory.ServerResourceTemplate {
	return inventory.NewServerResourceTemplate(
		ToolsetMetadataRepos,
		mcp.ResourceTemplate{
			Name:        "repository_content_tag",
			URITemplate: repositoryResourceTagContentURITemplate.Raw(),
			Description: t("RESOURCE_REPOSITORY_CONTENT_TAG_DESCRIPTION", "Repository Content for specific tag"),
			Icons:       octicons.Icons("tag"),
		},
		repositoryResourceContentsHandlerFunc(repositoryResourceTagContentURITemplate),
	)
}

// GetRepositoryResourcePrContent defines the resource template for getting repository content for a pull request.
func GetRepositoryResourcePrContent(t translations.TranslationHelperFunc) inventory.ServerResourceTemplate {
	return inventory.NewServerResourceTemplate(
		ToolsetMetadataRepos,
		mcp.ResourceTemplate{
			Name:        "repository_content_pr",
			URITemplate: repositoryResourcePrContentURITemplate.Raw(),
			Description: t("RESOURCE_REPOSITORY_CONTENT_PR_DESCRIPTION", "Repository Content for specific pull request"),
			Icons:       octicons.Icons("git-pull-request"),
		},
		repositoryResourceContentsHandlerFunc(repositoryResourcePrContentURITemplate),
	)
}

// repositoryResourceContentsHandlerFunc returns a ResourceHandlerFunc that creates handlers on-demand.
func repositoryResourceContentsHandlerFunc(resourceURITemplate *uritemplate.Template) inventory.ResourceHandlerFunc {
	return func(deps any) mcp.ResourceHandler {
		d := deps.(ToolDependencies)
		return RepositoryResourceContentsHandler(d, resourceURITemplate)
	}
}

// RepositoryResourceContentsHandler returns a handler function for repository content requests.
func RepositoryResourceContentsHandler(deps ToolDependencies, resourceURITemplate *uritemplate.Template) mcp.ResourceHandler {
	return func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		// Match the URI to extract parameters
		uriValues := resourceURITemplate.Match(request.Params.URI)
		if uriValues == nil {
			return nil, fmt.Errorf("failed to match URI: %s", request.Params.URI)
		}

		// Extract required vars
		owner := uriValues.Get("owner").String()
		repo := uriValues.Get("repo").String()

		if owner == "" {
			return nil, errors.New("owner is required")
		}

		if repo == "" {
			return nil, errors.New("repo is required")
		}

		pathValue := uriValues.Get("path")
		pathComponents := pathValue.List()
		var path string

		if len(pathComponents) == 0 {
			path = pathValue.String()
		} else {
			path = strings.Join(pathComponents, "/")
		}

		opts := &github.RepositoryContentGetOptions{}
		rawOpts := &raw.ContentOpts{}

		sha := uriValues.Get("sha").String()
		if sha != "" {
			opts.Ref = sha
			rawOpts.SHA = sha
		}

		branch := uriValues.Get("branch").String()
		if branch != "" {
			opts.Ref = "refs/heads/" + branch
			rawOpts.Ref = "refs/heads/" + branch
		}

		tag := uriValues.Get("tag").String()
		if tag != "" {
			opts.Ref = "refs/tags/" + tag
			rawOpts.Ref = "refs/tags/" + tag
		}

		prNumber := uriValues.Get("prNumber").String()
		if prNumber != "" {
			// fetch the PR from the API to get the latest commit and use SHA
			githubClient, err := deps.GetClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			prNum, err := strconv.Atoi(prNumber)
			if err != nil {
				return nil, fmt.Errorf("invalid pull request number: %w", err)
			}
			pr, _, err := githubClient.PullRequests.Get(ctx, owner, repo, prNum)
			if err != nil {
				return nil, fmt.Errorf("failed to get pull request: %w", err)
			}
			sha := pr.GetHead().GetSHA()
			rawOpts.SHA = sha
			opts.Ref = sha
		}
		//  if it's a directory
		if path == "" || strings.HasSuffix(path, "/") {
			return nil, fmt.Errorf("directories are not supported: %s", path)
		}
		rawClient, err := deps.GetRawClient(ctx)

		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub raw content client: %w", err)
		}

		resp, err := rawClient.GetRawContent(ctx, owner, repo, path, rawOpts)
		defer func() {
			_ = resp.Body.Close()
		}()
		// If the raw content is not found, we will fall back to the GitHub API (in case it is a directory)
		switch {
		case err != nil:
			return nil, fmt.Errorf("failed to get raw content: %w", err)
		case resp.StatusCode == http.StatusOK:
			ext := filepath.Ext(path)
			mimeType := resp.Header.Get("Content-Type")
			if ext == ".md" {
				mimeType = "text/markdown"
			} else if mimeType == "" {
				mimeType = mime.TypeByExtension(ext)
			}

			content, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read file content: %w", err)
			}

			switch {
			case strings.HasPrefix(mimeType, "text"), strings.HasPrefix(mimeType, "application"):
				return &mcp.ReadResourceResult{
					Contents: []*mcp.ResourceContents{
						{
							URI:      request.Params.URI,
							MIMEType: mimeType,
							Text:     string(content),
						},
					},
				}, nil
			default:
				var buf bytes.Buffer
				base64Encoder := base64.NewEncoder(base64.StdEncoding, &buf)
				_, err := base64Encoder.Write(content)
				if err != nil {
					return nil, fmt.Errorf("failed to base64 encode content: %w", err)
				}
				if err := base64Encoder.Close(); err != nil {
					return nil, fmt.Errorf("failed to close base64 encoder: %w", err)
				}

				return &mcp.ReadResourceResult{
					Contents: []*mcp.ResourceContents{
						{
							URI:      request.Params.URI,
							MIMEType: mimeType,
							Blob:     buf.Bytes(),
						},
					},
				}, nil
			}
		case resp.StatusCode != http.StatusNotFound:
			// If we got a response but it is not 200 OK, we return an error
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read response body: %w", err)
			}
			return nil, fmt.Errorf("failed to fetch raw content: %s", string(body))
		default:
			// This should be unreachable because GetContents should return an error if neither file nor directory content is found.
			return nil, errors.New("404 Not Found")
		}
	}
}
