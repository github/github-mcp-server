// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// LicensesService handles communication with the license related
// methods of the GitHub API.
//
// GitHub API docs: https://docs.github.com/rest/licenses?apiVersion=2022-11-28
type LicensesService service

// RepositoryLicense represents the license for a repository.
type RepositoryLicense struct {
	Name *string `json:"name,omitempty"`
	Path *string `json:"path,omitempty"`

	SHA         *string  `json:"sha,omitempty"`
	Size        *int     `json:"size,omitempty"`
	URL         *string  `json:"url,omitempty"`
	HTMLURL     *string  `json:"html_url,omitempty"`
	GitURL      *string  `json:"git_url,omitempty"`
	DownloadURL *string  `json:"download_url,omitempty"`
	Type        *string  `json:"type,omitempty"`
	Content     *string  `json:"content,omitempty"`
	Encoding    *string  `json:"encoding,omitempty"`
	License     *License `json:"license,omitempty"`
}

func (l RepositoryLicense) String() string {
	return Stringify(l)
}

// License represents an open source license.
type License struct {
	Key  *string `json:"key,omitempty"`
	Name *string `json:"name,omitempty"`
	URL  *string `json:"url,omitempty"`

	SPDXID         *string   `json:"spdx_id,omitempty"`
	HTMLURL        *string   `json:"html_url,omitempty"`
	Featured       *bool     `json:"featured,omitempty"`
	Description    *string   `json:"description,omitempty"`
	Implementation *string   `json:"implementation,omitempty"`
	Permissions    *[]string `json:"permissions,omitempty"`
	Conditions     *[]string `json:"conditions,omitempty"`
	Limitations    *[]string `json:"limitations,omitempty"`
	Body           *string   `json:"body,omitempty"`
}

func (l License) String() string {
	return Stringify(l)
}

// ListLicensesOptions specifies the optional parameters to the LicensesService.List method.
type ListLicensesOptions struct {
	Featured *bool `url:"featured,omitempty"`

	ListOptions
}

// List popular open source licenses.
//
// GitHub API docs: https://docs.github.com/rest/licenses/licenses?apiVersion=2022-11-28#get-all-commonly-used-licenses
//
//meta:operation GET /licenses
func (s *LicensesService) List(ctx context.Context, opts *ListLicensesOptions) ([]*License, *Response, error) {
	u := "licenses"
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var licenses []*License
	resp, err := s.client.Do(req, &licenses)
	if err != nil {
		return nil, resp, err
	}

	return licenses, resp, nil
}

// Get extended metadata for one license.
//
// GitHub API docs: https://docs.github.com/rest/licenses/licenses?apiVersion=2022-11-28#get-a-license
//
//meta:operation GET /licenses/{license}
func (s *LicensesService) Get(ctx context.Context, licenseName string) (*License, *Response, error) {
	u := fmt.Sprintf("licenses/%v", licenseName)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var license *License
	resp, err := s.client.Do(req, &license)
	if err != nil {
		return nil, resp, err
	}

	return license, resp, nil
}
