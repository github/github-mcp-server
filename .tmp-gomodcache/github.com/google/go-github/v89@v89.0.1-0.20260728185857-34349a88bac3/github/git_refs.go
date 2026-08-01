// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Reference represents a GitHub reference.
type Reference struct {
	// The name of the fully qualified reference, i.e.: `refs/heads/master`.
	Ref    *string    `json:"ref"`
	URL    *string    `json:"url"`
	Object *GitObject `json:"object"`
	NodeID *string    `json:"node_id,omitempty"`
}

func (r Reference) String() string {
	return Stringify(r)
}

// GitObject represents a Git object.
type GitObject struct {
	Type *string `json:"type"`
	SHA  *string `json:"sha"`
	URL  *string `json:"url"`
}

func (o GitObject) String() string {
	return Stringify(o)
}

// CreateRef represents the payload for creating a reference.
type CreateRef struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

// UpdateRef represents the payload for updating a reference.
type UpdateRef struct {
	SHA   string `json:"sha"`
	Force *bool  `json:"force,omitempty"`
}

// GetRef fetches a single reference in a repository.
// The ref must be formatted as `heads/<branch name>` for branches and `tags/<tag name>` for tags.
//
// GitHub API docs: https://docs.github.com/rest/git/refs?apiVersion=2022-11-28#get-a-reference
//
//meta:operation GET /repos/{owner}/{repo}/git/ref/{ref}
func (s *GitService) GetRef(ctx context.Context, owner, repo, ref string) (*Reference, *Response, error) {
	ref = strings.TrimPrefix(ref, "refs/")
	u := fmt.Sprintf("repos/%v/%v/git/ref/%v", owner, repo, refURLEscape(ref))
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var r *Reference
	resp, err := s.client.Do(req, &r)
	if err != nil {
		return nil, resp, err
	}

	return r, resp, nil
}

// refURLEscape escapes every path segment of the given ref. Those must
// not contain escaped "/" - as "%2F" - or github will not recognize it.
func refURLEscape(ref string) string {
	parts := strings.Split(ref, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// ListMatchingRefs lists references in a repository that match a supplied ref.
// The ref in the URL must be formatted as `heads/<branch name>` for branches and `tags/<tag name>` for tags.
// If the ref doesn't exist in the repository, but existing refs start with ref, they will be returned as an array.
// Use an empty ref to list all references.
//
// GitHub API docs: https://docs.github.com/rest/git/refs?apiVersion=2022-11-28#list-matching-references
//
//meta:operation GET /repos/{owner}/{repo}/git/matching-refs/{ref}
func (s *GitService) ListMatchingRefs(ctx context.Context, owner, repo, ref string) ([]*Reference, *Response, error) {
	ref = strings.TrimPrefix(ref, "refs/") // API expects no "refs/" prefix
	u := fmt.Sprintf("repos/%v/%v/git/matching-refs/%v", owner, repo, refURLEscape(ref))

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var rs []*Reference
	resp, err := s.client.Do(req, &rs)
	if err != nil {
		return nil, resp, err
	}

	return rs, resp, nil
}

// CreateRef creates a new ref in a repository.
//
// GitHub API docs: https://docs.github.com/rest/git/refs?apiVersion=2022-11-28#create-a-reference
//
//meta:operation POST /repos/{owner}/{repo}/git/refs
func (s *GitService) CreateRef(ctx context.Context, owner, repo string, body CreateRef) (*Reference, *Response, error) {
	if body.Ref == "" {
		return nil, nil, errors.New("ref must be provided")
	}

	if body.SHA == "" {
		return nil, nil, errors.New("sha must be provided")
	}

	// ensure the 'refs/' prefix is present
	body.Ref = "refs/" + strings.TrimPrefix(body.Ref, "refs/")

	u := fmt.Sprintf("repos/%v/%v/git/refs", owner, repo)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var r *Reference
	resp, err := s.client.Do(req, &r)
	if err != nil {
		return nil, resp, err
	}

	return r, resp, nil
}

// UpdateRef updates an existing ref in a repository.
//
// GitHub API docs: https://docs.github.com/rest/git/refs?apiVersion=2022-11-28#update-a-reference
//
//meta:operation PATCH /repos/{owner}/{repo}/git/refs/{ref}
func (s *GitService) UpdateRef(ctx context.Context, owner, repo, ref string, body UpdateRef) (*Reference, *Response, error) {
	if ref == "" {
		return nil, nil, errors.New("ref must be provided")
	}

	if body.SHA == "" {
		return nil, nil, errors.New("sha must be provided")
	}

	refPath := strings.TrimPrefix(ref, "refs/")
	u := fmt.Sprintf("repos/%v/%v/git/refs/%v", owner, repo, refURLEscape(refPath))
	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var r *Reference
	resp, err := s.client.Do(req, &r)
	if err != nil {
		return nil, resp, err
	}

	return r, resp, nil
}

// DeleteRef deletes a ref from a repository.
//
// GitHub API docs: https://docs.github.com/rest/git/refs?apiVersion=2022-11-28#delete-a-reference
//
//meta:operation DELETE /repos/{owner}/{repo}/git/refs/{ref}
func (s *GitService) DeleteRef(ctx context.Context, owner, repo, ref string) (*Response, error) {
	ref = strings.TrimPrefix(ref, "refs/")
	u := fmt.Sprintf("repos/%v/%v/git/refs/%v", owner, repo, refURLEscape(ref))
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
