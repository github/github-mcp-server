package github

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v72/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// GetRepositoryResourceContent defines the resource template and handler for getting repository content.
func GetRepositoryResourceContent(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.ResourceTemplate, server.ResourceTemplateHandlerFunc) {
	return mcp.NewResourceTemplate(
			"repo://{owner}/{repo}/contents{/path*}", // Resource template
			t("RESOURCE_REPOSITORY_CONTENT_DESCRIPTION", "Repository Content"),
		),
		RepositoryResourceContentsHandler(getClient)
}

// GetRepositoryResourceBranchContent defines the resource template and handler for getting repository content for a branch.
func GetRepositoryResourceBranchContent(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.ResourceTemplate, server.ResourceTemplateHandlerFunc) {
	return mcp.NewResourceTemplate(
			"repo://{owner}/{repo}/refs/heads/{branch}/contents{/path*}", // Resource template
			t("RESOURCE_REPOSITORY_CONTENT_BRANCH_DESCRIPTION", "Repository Content for specific branch"),
		),
		RepositoryResourceContentsHandler(getClient)
}

// GetRepositoryResourceCommitContent defines the resource template and handler for getting repository content for a commit.
func GetRepositoryResourceCommitContent(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.ResourceTemplate, server.ResourceTemplateHandlerFunc) {
	return mcp.NewResourceTemplate(
			"repo://{owner}/{repo}/sha/{sha}/contents{/path*}", // Resource template
			t("RESOURCE_REPOSITORY_CONTENT_COMMIT_DESCRIPTION", "Repository Content for specific commit"),
		),
		RepositoryResourceContentsHandler(getClient)
}

// GetRepositoryResourceTagContent defines the resource template and handler for getting repository content for a tag.
func GetRepositoryResourceTagContent(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.ResourceTemplate, server.ResourceTemplateHandlerFunc) {
	return mcp.NewResourceTemplate(
			"repo://{owner}/{repo}/refs/tags/{tag}/contents{/path*}", // Resource template
			t("RESOURCE_REPOSITORY_CONTENT_TAG_DESCRIPTION", "Repository Content for specific tag"),
		),
		RepositoryResourceContentsHandler(getClient)
}

// GetRepositoryResourcePrContent defines the resource template and handler for getting repository content for a pull request.
func GetRepositoryResourcePrContent(getClient GetClientFn, t translations.TranslationHelperFunc) (mcp.ResourceTemplate, server.ResourceTemplateHandlerFunc) {
	return mcp.NewResourceTemplate(
			"repo://{owner}/{repo}/refs/pull/{prNumber}/head/contents{/path*}", // Resource template
			t("RESOURCE_REPOSITORY_CONTENT_PR_DESCRIPTION", "Repository Content for specific pull request"),
		),
		RepositoryResourceContentsHandler(getClient)
}

// RepositoryResourceContentsHandler returns a handler function for repository content requests.
func RepositoryResourceContentsHandler(getClient GetClientFn) func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// the matcher will give []string with one element
		// https://github.com/mark3labs/mcp-go/pull/54
		o, ok := request.Params.Arguments["owner"].([]string)
		if !ok || len(o) == 0 {
			return nil, errors.New("owner is required")
		}
		owner := o[0]

		r, ok := request.Params.Arguments["repo"].([]string)
		if !ok || len(r) == 0 {
			return nil, errors.New("repo is required")
		}
		repo := r[0]

		// path should be a joined list of the path parts
		path := ""
		p, ok := request.Params.Arguments["path"].([]string)
		if ok {
			path = strings.Join(p, "/")
		}

		opts := &github.RepositoryContentGetOptions{}

		sha, ok := request.Params.Arguments["sha"].([]string)
		if ok && len(sha) > 0 {
			opts.Ref = sha[0]
		}

		branch, ok := request.Params.Arguments["branch"].([]string)
		if ok && len(branch) > 0 {
			opts.Ref = "refs/heads/" + branch[0]
		}

		tag, ok := request.Params.Arguments["tag"].([]string)
		if ok && len(tag) > 0 {
			opts.Ref = "refs/tags/" + tag[0]
		}
		prNumber, ok := request.Params.Arguments["prNumber"].([]string)
		if ok && len(prNumber) > 0 {
			opts.Ref = "refs/pull/" + prNumber[0] + "/head"
		}

		client, err := getClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}
		fileContent, directoryContent, _, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
		if err != nil {
			return nil, err
		}

		if directoryContent != nil {
			var resources []mcp.ResourceContents
			for _, entry := range directoryContent {
				mimeType := "text/directory"
				if entry.GetType() == "file" {
					// this is system dependent, and a best guess
					ext := filepath.Ext(entry.GetName())
					mimeType = mime.TypeByExtension(ext)
					if ext == ".md" {
						mimeType = "text/markdown"
					}
				}
				resources = append(resources, mcp.TextResourceContents{
					URI:      entry.GetHTMLURL(),
					MIMEType: mimeType,
					Text:     entry.GetName(),
				})

			}
			return resources, nil

		}
		if fileContent != nil {
			if fileContent.Content != nil {
				// download the file content from fileContent.GetDownloadURL() and use the content-type header to determine the MIME type
				// and return the content as a blob unless it is a text file, where you can return the content as text
				req, err := http.NewRequest("GET", fileContent.GetDownloadURL(), nil)
				if err != nil {
					return nil, fmt.Errorf("failed to create request: %w", err)
				}

				resp, err := client.Client().Do(req)
				if err != nil {
					return nil, fmt.Errorf("failed to send request: %w", err)
				}
				defer func() { _ = resp.Body.Close() }()

				if resp.StatusCode != http.StatusOK {
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return nil, fmt.Errorf("failed to read response body: %w", err)
					}
					return nil, fmt.Errorf("failed to fetch file content: %s", string(body))
				}

				ext := filepath.Ext(fileContent.GetName())
				mimeType := resp.Header.Get("Content-Type")
				if ext == ".md" {
					mimeType = "text/markdown"
				} else if mimeType == "" {
					// backstop to the file extension if the content type is not set
					mimeType = mime.TypeByExtension(filepath.Ext(fileContent.GetName()))
				}

				// if the content is a string, return it as text
				if strings.HasPrefix(mimeType, "text") {
					content, err := io.ReadAll(resp.Body)
					if err != nil {
						return nil, fmt.Errorf("failed to parse the response body: %w", err)
					}

					return []mcp.ResourceContents{
						mcp.TextResourceContents{
							URI:      request.Params.URI,
							MIMEType: mimeType,
							Text:     string(content),
						},
					}, nil
				}
				// otherwise, read the content and encode it as base64
				decodedContent, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to parse the response body: %w", err)
				}

				return []mcp.ResourceContents{
					mcp.BlobResourceContents{
						URI:      request.Params.URI,
						MIMEType: mimeType,
						Blob:     base64.StdEncoding.EncodeToString(decodedContent), // Encode content as Base64
					},
				}, nil
			}
		}

		// This should be unreachable because GetContents should return an error if neither file nor directory content is found.
		return nil, errors.New("no repository resource content found")
	}
}
