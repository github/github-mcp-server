// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"net/url"
)

// ListPackages lists the packages for a user. Passing the empty string for "user" will
// list packages for the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#list-packages-for-a-user
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#list-packages-for-the-authenticated-users-namespace
//
//meta:operation GET /user/packages
//meta:operation GET /users/{username}/packages
func (s *UsersService) ListPackages(ctx context.Context, user string, opts *PackageListOptions) ([]*Package, *Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages", user)
	} else {
		u = "user/packages"
	}
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var packages []*Package
	resp, err := s.client.Do(req, &packages)
	if err != nil {
		return nil, resp, err
	}

	return packages, resp, nil
}

// GetPackage gets a package by name for a user. Passing the empty string for "user" will
// get the package for the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#get-a-package-for-a-user
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#get-a-package-for-the-authenticated-user
//
//meta:operation GET /user/packages/{package_type}/{package_name}
//meta:operation GET /users/{username}/packages/{package_type}/{package_name}
func (s *UsersService) GetPackage(ctx context.Context, user, packageType, packageName string) (*Package, *Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v", user, packageType, url.PathEscape(packageName))
	} else {
		u = fmt.Sprintf("user/packages/%v/%v", packageType, url.PathEscape(packageName))
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var pack *Package
	resp, err := s.client.Do(req, &pack)
	if err != nil {
		return nil, resp, err
	}

	return pack, resp, nil
}

// DeletePackage deletes a package from a user. Passing the empty string for "user" will
// delete the package for the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#delete-a-package-for-a-user
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#delete-a-package-for-the-authenticated-user
//
//meta:operation DELETE /user/packages/{package_type}/{package_name}
//meta:operation DELETE /users/{username}/packages/{package_type}/{package_name}
func (s *UsersService) DeletePackage(ctx context.Context, user, packageType, packageName string) (*Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v", user, packageType, packageName)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v", packageType, packageName)
	}

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// RestorePackage restores a package to a user. Passing the empty string for "user" will
// restore the package for the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#restore-a-package-for-a-user
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#restore-a-package-for-the-authenticated-user
//
//meta:operation POST /user/packages/{package_type}/{package_name}/restore
//meta:operation POST /users/{username}/packages/{package_type}/{package_name}/restore
func (s *UsersService) RestorePackage(ctx context.Context, user, packageType, packageName string) (*Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v/restore", user, packageType, packageName)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v/restore", packageType, packageName)
	}

	req, err := s.client.NewRequest(ctx, "POST", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// ListPackageVersionsOptions specifies the optional parameters to the UsersService.ListPackageVersions.
type ListPackageVersionsOptions struct {
	// State of package either "active" or "deleted".
	State string `url:"state,omitempty"`

	ListOptions
}

// ListPackageVersions gets all versions of a package for the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#list-package-versions-for-a-package-owned-by-the-authenticated-user
//
//meta:operation GET /user/packages/{package_type}/{package_name}/versions
func (s *UsersService) ListPackageVersions(ctx context.Context, packageType, packageName string, opts *ListPackageVersionsOptions) ([]*PackageVersion, *Response, error) {
	u := fmt.Sprintf("user/packages/%v/%v/versions", packageType, packageName)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var versions []*PackageVersion
	resp, err := s.client.Do(req, &versions)
	if err != nil {
		return nil, resp, err
	}

	return versions, resp, nil
}

// ListUserPackageVersions returns package versions for a public package owned by a specified user.
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#list-package-versions-for-a-package-owned-by-a-user
//
//meta:operation GET /users/{username}/packages/{package_type}/{package_name}/versions
func (s *UsersService) ListUserPackageVersions(ctx context.Context, user, packageType, packageName string) ([]*PackageVersion, *Response, error) {
	u := fmt.Sprintf("users/%v/packages/%v/%v/versions", user, packageType, packageName)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var versions []*PackageVersion
	resp, err := s.client.Do(req, &versions)
	if err != nil {
		return nil, resp, err
	}

	return versions, resp, nil
}

// PackageGetVersion gets a specific version of a package for a user. Passing the empty string for "user" will
// get the version for the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#get-a-package-version-for-a-user
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#get-a-package-version-for-the-authenticated-user
//
//meta:operation GET /user/packages/{package_type}/{package_name}/versions/{package_version_id}
//meta:operation GET /users/{username}/packages/{package_type}/{package_name}/versions/{package_version_id}
func (s *UsersService) PackageGetVersion(ctx context.Context, user, packageType, packageName string, packageVersionID int64) (*PackageVersion, *Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v/versions/%v", user, packageType, packageName, packageVersionID)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v/versions/%v", packageType, packageName, packageVersionID)
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var version *PackageVersion
	resp, err := s.client.Do(req, &version)
	if err != nil {
		return nil, resp, err
	}

	return version, resp, nil
}

// PackageDeleteVersion deletes a package version for a user. Passing the empty string for "user" will
// delete the version for the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#delete-a-package-version-for-the-authenticated-user
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#delete-package-version-for-a-user
//
//meta:operation DELETE /user/packages/{package_type}/{package_name}/versions/{package_version_id}
//meta:operation DELETE /users/{username}/packages/{package_type}/{package_name}/versions/{package_version_id}
func (s *UsersService) PackageDeleteVersion(ctx context.Context, user, packageType, packageName string, packageVersionID int64) (*Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v/versions/%v", user, packageType, packageName, packageVersionID)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v/versions/%v", packageType, packageName, packageVersionID)
	}

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// PackageRestoreVersion restores a package version to a user. Passing the empty string for "user" will
// restore the version for the authenticated user.
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#restore-a-package-version-for-the-authenticated-user
//
// GitHub API docs: https://docs.github.com/rest/packages/packages?apiVersion=2022-11-28#restore-package-version-for-a-user
//
//meta:operation POST /user/packages/{package_type}/{package_name}/versions/{package_version_id}/restore
//meta:operation POST /users/{username}/packages/{package_type}/{package_name}/versions/{package_version_id}/restore
func (s *UsersService) PackageRestoreVersion(ctx context.Context, user, packageType, packageName string, packageVersionID int64) (*Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v/versions/%v/restore", user, packageType, packageName, packageVersionID)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v/versions/%v/restore", packageType, packageName, packageVersionID)
	}

	req, err := s.client.NewRequest(ctx, "POST", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
