// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/internal/mcpgodebug"
	"github.com/modelcontextprotocol/go-sdk/internal/util"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/yosida95/uritemplate/v3"
)

// A serverResource associates a Resource with its handler.
type serverResource struct {
	resource *Resource
	handler  ResourceHandler
}

// A serverResourceTemplate associates a ResourceTemplate with its handler.
type serverResourceTemplate struct {
	resourceTemplate *ResourceTemplate
	handler          ResourceHandler
}

// A ResourceHandler is a function that reads a resource.
// It will be called when the client calls [ClientSession.ReadResource].
// If it cannot find the resource, it should return the result of calling [ResourceNotFoundError].
type ResourceHandler func(context.Context, *ReadResourceRequest) (*ReadResourceResult, error)

// customresnotfounderrcode is a compatibility parameter that restores the
// pre-1.7.0 behavior of [ResourceNotFoundError] and [CodeResourceNotFound],
// where the error code was a custom -32002. See the documentation for the mcpgodebug
// package for instructions on how to enable it.
// The option will be removed in the future version of the SDK.
var customresnotfounderrcode = mcpgodebug.Value("customresnotfounderrcode")

func init() {
	if customresnotfounderrcode == "1" {
		CodeResourceNotFound = -32002
	}
}

// ResourceNotFoundError returns an error indicating that a resource being read could
// not be found.
//
// By default, the error code is -32602 (Invalid Params), as specified in the
// MCP specification (SEP-2164). To restore the pre-1.7.0 release behavior where the
// error code was -32002, set MCPGODEBUG=customresnotfounderrcode=1.
func ResourceNotFoundError(uri string) error {
	return &jsonrpc.Error{
		Code:    CodeResourceNotFound,
		Message: "Resource not found",
		Data:    json.RawMessage(fmt.Sprintf(`{"uri":%q}`, uri)),
	}
}

// readFileResource reads from the filesystem at a URI relative to dirFilepath, respecting
// the roots.
// dirFilepath and rootFilepaths are absolute filesystem paths.
func readFileResource(rawURI, dirFilepath string, rootFilepaths []string) ([]byte, error) {
	uriFilepath, err := computeURIFilepath(rawURI, dirFilepath, rootFilepaths)
	if err != nil {
		return nil, err
	}

	var data []byte
	err = withFile(dirFilepath, uriFilepath, func(f *os.File) error {
		var err error
		data, err = io.ReadAll(f)
		return err
	})
	if os.IsNotExist(err) {
		err = ResourceNotFoundError(rawURI)
	}
	return data, err
}

// computeURIFilepath returns a path relative to dirFilepath.
// The dirFilepath and rootFilepaths are absolute file paths.
func computeURIFilepath(rawURI, dirFilepath string, rootFilepaths []string) (string, error) {
	// We use "file path" to mean a filesystem path.
	uri, err := url.Parse(rawURI)
	if err != nil {
		return "", err
	}
	if uri.Scheme != "file" {
		return "", fmt.Errorf("URI is not a file: %s", uri)
	}
	if uri.Path == "" {
		// A more specific error than the one below, to catch the
		// common mistake "file://foo".
		return "", errors.New("empty path")
	}
	// The URI's path is interpreted relative to dirFilepath, and in the local filesystem.
	// It must not try to escape its directory.
	uriFilepathRel, err := filepath.Localize(strings.TrimPrefix(uri.Path, "/"))
	if err != nil {
		return "", fmt.Errorf("%q cannot be localized: %w", uriFilepathRel, err)
	}

	// Check roots, if there are any.
	if len(rootFilepaths) > 0 {
		// To check against the roots, we need an absolute file path, not relative to the directory.
		// uriFilepath is local, so the joined path is under dirFilepath.
		uriFilepathAbs := filepath.Join(dirFilepath, uriFilepathRel)
		rootOK := false
		// Check that the requested file path is under some root.
		// Since both paths are absolute, that's equivalent to filepath.Rel constructing
		// a local path.
		for _, rootFilepathAbs := range rootFilepaths {
			if rel, err := filepath.Rel(rootFilepathAbs, uriFilepathAbs); err == nil && filepath.IsLocal(rel) {
				rootOK = true
				break
			}
		}
		if !rootOK {
			return "", fmt.Errorf("URI path %q is not under any root", uriFilepathAbs)
		}
	}
	return uriFilepathRel, nil
}

// withFile calls f on the file at join(dir, rel),
// protecting against path traversal attacks.
func withFile(dir, rel string, f func(*os.File) error) (err error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer r.Close()
	file, err := r.Open(rel)
	if err != nil {
		return err
	}
	// Record error, in case f writes.
	defer func() { err = errors.Join(err, file.Close()) }()
	return f(file)
}

// fileRoots transforms the Roots obtained from the client into absolute paths on
// the local filesystem.
// TODO(jba): expose this functionality to user ResourceHandlers,
// so they don't have to repeat it.
func fileRoots(rawRoots []*Root) ([]string, error) {
	var fileRoots []string
	for _, r := range rawRoots {
		fr, err := fileRoot(r)
		if err != nil {
			return nil, err
		}
		fileRoots = append(fileRoots, fr)
	}
	return fileRoots, nil
}

// fileRoot returns the absolute path for Root.
func fileRoot(root *Root) (_ string, err error) {
	defer util.Wrapf(&err, "root %q", root.URI)

	// Convert to absolute file path.
	rurl, err := url.Parse(root.URI)
	if err != nil {
		return "", err
	}
	if rurl.Scheme != "file" {
		return "", errors.New("not a file URI")
	}
	if rurl.Path == "" {
		// A more specific error than the one below, to catch the
		// common mistake "file://foo".
		return "", errors.New("empty path")
	}
	// We don't want Localize here: we want an absolute path, which is not local.
	fileRoot := filepath.Clean(filepath.FromSlash(rurl.Path))
	if !filepath.IsAbs(fileRoot) {
		return "", errors.New("not an absolute path")
	}
	return fileRoot, nil
}

// Matches reports whether the receiver's uri template matches the uri.
func (sr *serverResourceTemplate) Matches(uri string) bool {
	tmpl, err := uritemplate.New(sr.resourceTemplate.URITemplate)
	if err != nil {
		return false
	}
	return tmpl.Regexp().MatchString(uri)
}
