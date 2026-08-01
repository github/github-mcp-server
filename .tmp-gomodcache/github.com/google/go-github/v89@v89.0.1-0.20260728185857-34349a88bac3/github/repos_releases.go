// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// RepositoryRelease represents a GitHub release in a repository.
type RepositoryRelease struct {
	TagName         string          `json:"tag_name"`
	TargetCommitish string          `json:"target_commitish"`
	Name            *string         `json:"name"`
	Body            *string         `json:"body,omitempty"`
	Draft           bool            `json:"draft"`
	Prerelease      bool            `json:"prerelease"`
	Immutable       *bool           `json:"immutable,omitempty"`
	ID              int64           `json:"id"`
	CreatedAt       Timestamp       `json:"created_at"`
	PublishedAt     *Timestamp      `json:"published_at"`
	UpdatedAt       *Timestamp      `json:"updated_at,omitempty"`
	URL             string          `json:"url"`
	HTMLURL         string          `json:"html_url"`
	AssetsURL       string          `json:"assets_url"`
	Assets          []*ReleaseAsset `json:"assets"`
	UploadURL       string          `json:"upload_url"`
	ZipballURL      *string         `json:"zipball_url"`
	TarballURL      *string         `json:"tarball_url"`
	Author          *User           `json:"author"`
	NodeID          string          `json:"node_id"`
	BodyHTML        *string         `json:"body_html,omitempty"`
	BodyText        *string         `json:"body_text,omitempty"`
	MentionsCount   *int            `json:"mentions_count,omitempty"`
	DiscussionURL   *string         `json:"discussion_url,omitempty"`
	Reactions       *Reactions      `json:"reactions,omitempty"`
}

func (r RepositoryRelease) String() string {
	return Stringify(r)
}

// RepositoryReleaseNotes represents a GitHub-generated release notes.
type RepositoryReleaseNotes struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

// GenerateNotesRequest represents the request to generate release notes.
type GenerateNotesRequest struct {
	TagName               string  `json:"tag_name"`
	PreviousTagName       *string `json:"previous_tag_name,omitempty"`
	TargetCommitish       *string `json:"target_commitish,omitempty"`
	ConfigurationFilePath *string `json:"configuration_file_path,omitempty"`
}

// ReleaseAsset represents a GitHub release asset in a repository.
type ReleaseAsset struct {
	ID                 *int64     `json:"id,omitempty"`
	URL                *string    `json:"url,omitempty"`
	Name               *string    `json:"name,omitempty"`
	Label              *string    `json:"label,omitempty"`
	State              *string    `json:"state,omitempty"`
	ContentType        *string    `json:"content_type,omitempty"`
	Size               *int       `json:"size,omitempty"`
	DownloadCount      *int       `json:"download_count,omitempty"`
	CreatedAt          *Timestamp `json:"created_at,omitempty"`
	UpdatedAt          *Timestamp `json:"updated_at,omitempty"`
	BrowserDownloadURL *string    `json:"browser_download_url,omitempty"`
	Uploader           *User      `json:"uploader,omitempty"`
	NodeID             *string    `json:"node_id,omitempty"`
	Digest             *string    `json:"digest,omitempty"`
}

// UpdateReleaseAssetRequest represents the request to update a release asset.
type UpdateReleaseAssetRequest struct {
	Name  *string `json:"name,omitempty"`
	Label *string `json:"label,omitempty"`
	State *string `json:"state,omitempty"`
}

func (r ReleaseAsset) String() string {
	return Stringify(r)
}

// ListReleases lists the releases for a repository.
//
// GitHub API docs: https://docs.github.com/rest/releases/releases?apiVersion=2022-11-28#list-releases
//
//meta:operation GET /repos/{owner}/{repo}/releases
func (s *RepositoriesService) ListReleases(ctx context.Context, owner, repo string, opts *ListOptions) ([]*RepositoryRelease, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases", owner, repo)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var releases []*RepositoryRelease
	resp, err := s.client.Do(req, &releases)
	if err != nil {
		return nil, resp, err
	}
	return releases, resp, nil
}

// GetRelease fetches a single release.
//
// GitHub API docs: https://docs.github.com/rest/releases/releases?apiVersion=2022-11-28#get-a-release
//
//meta:operation GET /repos/{owner}/{repo}/releases/{release_id}
func (s *RepositoriesService) GetRelease(ctx context.Context, owner, repo string, id int64) (*RepositoryRelease, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases/%v", owner, repo, id)
	return s.getSingleRelease(ctx, u)
}

// GetLatestRelease fetches the latest published release for the repository.
//
// GitHub API docs: https://docs.github.com/rest/releases/releases?apiVersion=2022-11-28#get-the-latest-release
//
//meta:operation GET /repos/{owner}/{repo}/releases/latest
func (s *RepositoriesService) GetLatestRelease(ctx context.Context, owner, repo string) (*RepositoryRelease, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases/latest", owner, repo)
	return s.getSingleRelease(ctx, u)
}

// GetReleaseByTag fetches a release with the specified tag.
//
// GitHub API docs: https://docs.github.com/rest/releases/releases?apiVersion=2022-11-28#get-a-release-by-tag-name
//
//meta:operation GET /repos/{owner}/{repo}/releases/tags/{tag}
func (s *RepositoriesService) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*RepositoryRelease, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases/tags/%v", owner, repo, tag)
	return s.getSingleRelease(ctx, u)
}

// GenerateReleaseNotes generates the release notes for the given tag.
//
// GitHub API docs: https://docs.github.com/rest/releases/releases?apiVersion=2022-11-28#generate-release-notes-content-for-a-release
//
//meta:operation POST /repos/{owner}/{repo}/releases/generate-notes
func (s *RepositoriesService) GenerateReleaseNotes(ctx context.Context, owner, repo string, body GenerateNotesRequest) (*RepositoryReleaseNotes, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases/generate-notes", owner, repo)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var r *RepositoryReleaseNotes
	resp, err := s.client.Do(req, &r)
	if err != nil {
		return nil, resp, err
	}

	return r, resp, nil
}

func (s *RepositoriesService) getSingleRelease(ctx context.Context, url string) (*RepositoryRelease, *Response, error) {
	req, err := s.client.NewRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	var release *RepositoryRelease
	resp, err := s.client.Do(req, &release)
	if err != nil {
		return nil, resp, err
	}

	return release, resp, nil
}

// CreateReleaseRequest represents a request to create a release in a repository.
type CreateReleaseRequest struct {
	TagName         string  `json:"tag_name"`
	TargetCommitish *string `json:"target_commitish,omitempty"`
	Name            *string `json:"name,omitempty"`
	Body            *string `json:"body,omitempty"`
	Draft           *bool   `json:"draft,omitempty"`
	Prerelease      *bool   `json:"prerelease,omitempty"`
	// MakeLatest can be one of: "true", "false", or "legacy".
	MakeLatest             *string `json:"make_latest,omitempty"`
	DiscussionCategoryName *string `json:"discussion_category_name,omitempty"`
	GenerateReleaseNotes   *bool   `json:"generate_release_notes,omitempty"`
}

// UpdateReleaseRequest represents a request to update a release in a repository.
type UpdateReleaseRequest struct {
	TagName         *string `json:"tag_name,omitempty"`
	TargetCommitish *string `json:"target_commitish,omitempty"`
	Name            *string `json:"name,omitempty"`
	Body            *string `json:"body,omitempty"`
	Draft           *bool   `json:"draft,omitempty"`
	Prerelease      *bool   `json:"prerelease,omitempty"`
	// MakeLatest can be one of: "true", "false", or "legacy".
	MakeLatest             *string `json:"make_latest,omitempty"`
	DiscussionCategoryName *string `json:"discussion_category_name,omitempty"`
}

// CreateRelease adds a new release for a repository.
//
// GitHub API docs: https://docs.github.com/rest/releases/releases?apiVersion=2022-11-28#create-a-release
//
//meta:operation POST /repos/{owner}/{repo}/releases
func (s *RepositoriesService) CreateRelease(ctx context.Context, owner, repo string, body CreateReleaseRequest) (*RepositoryRelease, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases", owner, repo)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var r *RepositoryRelease
	resp, err := s.client.Do(req, &r)
	if err != nil {
		return nil, resp, err
	}

	return r, resp, nil
}

// UpdateRelease updates a repository release.
//
// GitHub API docs: https://docs.github.com/rest/releases/releases?apiVersion=2022-11-28#update-a-release
//
//meta:operation PATCH /repos/{owner}/{repo}/releases/{release_id}
func (s *RepositoriesService) UpdateRelease(ctx context.Context, owner, repo string, id int64, body UpdateReleaseRequest) (*RepositoryRelease, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases/%v", owner, repo, id)

	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var r *RepositoryRelease
	resp, err := s.client.Do(req, &r)
	if err != nil {
		return nil, resp, err
	}

	return r, resp, nil
}

// DeleteRelease delete a single release from a repository.
//
// GitHub API docs: https://docs.github.com/rest/releases/releases?apiVersion=2022-11-28#delete-a-release
//
//meta:operation DELETE /repos/{owner}/{repo}/releases/{release_id}
func (s *RepositoriesService) DeleteRelease(ctx context.Context, owner, repo string, id int64) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases/%v", owner, repo, id)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}
	return s.client.Do(req, nil)
}

// ListReleaseAssets lists the release's assets.
//
// GitHub API docs: https://docs.github.com/rest/releases/assets?apiVersion=2022-11-28#list-release-assets
//
//meta:operation GET /repos/{owner}/{repo}/releases/{release_id}/assets
func (s *RepositoriesService) ListReleaseAssets(ctx context.Context, owner, repo string, id int64, opts *ListOptions) ([]*ReleaseAsset, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases/%v/assets", owner, repo, id)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var assets []*ReleaseAsset
	resp, err := s.client.Do(req, &assets)
	if err != nil {
		return nil, resp, err
	}
	return assets, resp, nil
}

// GetReleaseAsset fetches a single release asset.
//
// GitHub API docs: https://docs.github.com/rest/releases/assets?apiVersion=2022-11-28#get-a-release-asset
//
//meta:operation GET /repos/{owner}/{repo}/releases/assets/{asset_id}
func (s *RepositoriesService) GetReleaseAsset(ctx context.Context, owner, repo string, id int64) (*ReleaseAsset, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases/assets/%v", owner, repo, id)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var asset *ReleaseAsset
	resp, err := s.client.Do(req, &asset)
	if err != nil {
		return nil, resp, err
	}

	return asset, resp, nil
}

// DownloadReleaseAsset downloads a release asset or returns a redirect URL.
//
// DownloadReleaseAsset returns an io.ReadCloser that reads the contents of the
// specified release asset. It is the caller's responsibility to close the ReadCloser.
// If a redirect is returned, the redirect URL will be returned as a string instead
// of the io.ReadCloser. Exactly one of rc and redirectURL will be zero.
//
// followRedirectsClient can be passed to download the asset from a redirected
// location. Specifying any http.Client is possible, but passing http.DefaultClient
// is recommended, except when the specified repository is private, in which case
// it's necessary to pass an http.Client that performs authenticated requests.
// If nil is passed the redirectURL will be returned instead.
//
// GitHub API docs: https://docs.github.com/rest/releases/assets?apiVersion=2022-11-28#get-a-release-asset
//
//meta:operation GET /repos/{owner}/{repo}/releases/assets/{asset_id}
func (s *RepositoriesService) DownloadReleaseAsset(ctx context.Context, owner, repo string, id int64, followRedirectsClient *http.Client) (rc io.ReadCloser, redirectURL string, err error) {
	u := fmt.Sprintf("repos/%v/%v/releases/assets/%v", owner, repo, id)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", defaultMediaType)

	loc, resp, err := s.client.bareDoUntilFound(req, 10)
	if err != nil {
		return nil, "", err
	}

	// No redirect, stream the response body directly.
	if loc == nil {
		return resp.Body, "", nil
	}

	// Close body as it's not needed when following redirects or returning the redirect URL.
	_ = resp.Body.Close()

	// Got a redirect URL.
	redirectStr := loc.String()
	if followRedirectsClient != nil {
		rc, err := s.downloadReleaseAssetFromURL(ctx, followRedirectsClient, redirectStr)
		return rc, "", err
	}

	return nil, redirectStr, nil
}

func (s *RepositoriesService) downloadReleaseAssetFromURL(ctx context.Context, followRedirectsClient *http.Client, url string) (rc io.ReadCloser, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", defaultMediaType)
	resp, err := followRedirectsClient.Do(req)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	return resp.Body, nil
}

// UpdateReleaseAsset updates a repository release asset.
//
// GitHub API docs: https://docs.github.com/rest/releases/assets?apiVersion=2022-11-28#update-a-release-asset
//
//meta:operation PATCH /repos/{owner}/{repo}/releases/assets/{asset_id}
func (s *RepositoriesService) UpdateReleaseAsset(ctx context.Context, owner, repo string, id int64, body UpdateReleaseAssetRequest) (*ReleaseAsset, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases/assets/%v", owner, repo, id)

	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var asset *ReleaseAsset
	resp, err := s.client.Do(req, &asset)
	if err != nil {
		return nil, resp, err
	}

	return asset, resp, nil
}

// DeleteReleaseAsset delete a single release asset from a repository.
//
// GitHub API docs: https://docs.github.com/rest/releases/assets?apiVersion=2022-11-28#delete-a-release-asset
//
//meta:operation DELETE /repos/{owner}/{repo}/releases/assets/{asset_id}
func (s *RepositoriesService) DeleteReleaseAsset(ctx context.Context, owner, repo string, id int64) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/releases/assets/%v", owner, repo, id)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}
	return s.client.Do(req, nil)
}

// UploadReleaseAsset creates an asset by uploading a file into a release repository.
// To upload assets that cannot be represented by an os.File, call NewUploadRequest directly.
//
// GitHub API docs: https://docs.github.com/rest/releases/assets?apiVersion=2022-11-28#upload-a-release-asset
//
//meta:operation POST /repos/{owner}/{repo}/releases/{release_id}/assets
func (s *RepositoriesService) UploadReleaseAsset(ctx context.Context, owner, repo string, id int64, opts *UploadOptions, file *os.File) (*ReleaseAsset, *Response, error) {
	if file == nil {
		return nil, nil, errors.New("file must be provided")
	}

	u := fmt.Sprintf("repos/%v/%v/releases/%v/assets", owner, repo, id)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if stat.IsDir() {
		return nil, nil, errors.New("the asset to upload can't be a directory")
	}

	mediaType := mime.TypeByExtension(filepath.Ext(file.Name()))
	if opts.MediaType != "" {
		mediaType = opts.MediaType
	}

	req, err := s.client.NewUploadRequest(ctx, u, file, stat.Size(), mediaType)
	if err != nil {
		return nil, nil, err
	}

	var asset *ReleaseAsset
	resp, err := s.client.Do(req, &asset)
	if err != nil {
		return nil, resp, err
	}

	return asset, resp, nil
}

// UploadReleaseAssetFromRelease uploads an asset using the UploadURL that's embedded
// in a RepositoryRelease object.
//
// This is a convenience wrapper that extracts the release.UploadURL (which is usually
// templated like "https://uploads.github.com/.../assets{?name,label}") and uploads
// the provided data (reader + size) using the existing upload helpers.
//
// GitHub API docs: https://docs.github.com/rest/releases/assets?apiVersion=2022-11-28#upload-a-release-asset
//
//meta:operation POST /repos/{owner}/{repo}/releases/{release_id}/assets
func (s *RepositoriesService) UploadReleaseAssetFromRelease(
	ctx context.Context,
	release *RepositoryRelease,
	opts *UploadOptions,
	reader io.Reader,
	size int64,
) (*ReleaseAsset, *Response, error) {
	if release == nil || release.UploadURL == "" {
		return nil, nil, errors.New("release UploadURL must be provided")
	}
	if reader == nil {
		return nil, nil, errors.New("reader must be provided")
	}
	if size < 0 {
		return nil, nil, errors.New("size must be >= 0")
	}

	// Strip URI-template portion (e.g. "{?name,label}") if present.
	uploadURL := release.UploadURL
	if idx := strings.Index(uploadURL, "{"); idx != -1 {
		uploadURL = uploadURL[:idx]
	}

	// If this is a *relative* URL (no scheme), normalize it by trimming a leading "/"
	// so it works with Client.BaseURL path prefixes (e.g. "/api-v3/").
	if !strings.HasPrefix(uploadURL, "http://") && !strings.HasPrefix(uploadURL, "https://") {
		uploadURL = strings.TrimPrefix(uploadURL, "/")
	}

	// addOptions will append name/label query params (same behavior as UploadReleaseAsset).
	u, err := addOptions(uploadURL, opts)
	if err != nil {
		return nil, nil, err
	}

	// determine media type
	mediaType := defaultMediaType
	if opts != nil {
		switch {
		case opts.MediaType != "":
			mediaType = opts.MediaType
		case opts.Name != "":
			if ext := filepath.Ext(opts.Name); ext != "" {
				if mt := mime.TypeByExtension(ext); mt != "" {
					mediaType = mt
				}
			}
		}
	}

	req, err := s.client.NewUploadRequest(ctx, u, reader, size, mediaType)
	if err != nil {
		return nil, nil, err
	}

	var asset *ReleaseAsset
	resp, err := s.client.Do(req, &asset)
	if err != nil {
		return nil, resp, err
	}

	return asset, resp, nil
}
