// Package raw provides a client for interacting with the GitHub raw file API
package raw

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	gogithub "github.com/google/go-github/v92/github"
)

// errPathTraversal is returned when an owner, repo, ref/sha, or path
// component used to build a raw content URL contains a ".." path segment,
// either literally or via percent-decoding.
var errPathTraversal = errors.New(`raw: path segment ".." is not allowed`)

// rejectPathTraversal reports an error if any "/"-separated segment of the
// given components is, or decodes to, "..". url.URL.JoinPath cleans the
// joined path (resolving ".." segments) before producing the final URL, so a
// ".." segment anywhere in owner, repo, ref/sha, or path could otherwise
// rebind the resulting raw.githubusercontent.com URL to a different owner,
// repository, or ref than the one requested.
func rejectPathTraversal(components ...string) error {
	for _, component := range components {
		for segment := range strings.SplitSeq(component, "/") {
			if err := rejectSegment(segment); err != nil {
				return err
			}
		}
	}
	return nil
}

// rejectSegment reports an error if segment is, or decodes to, "..". A
// percent-encoded separator (e.g. "%2f") can appear inside a single
// "/"-separated segment and only becomes a "/" once decoded, revealing new
// subsegments (e.g. "%2e%2e%2fsecret.txt" decodes to "../secret.txt"). To
// catch that, whenever decoding changes the segment, the decoded form is
// split on "/" again and each subsegment is checked recursively, so
// traversal segments introduced by one or more layers of percent-decoding
// are rejected regardless of where the encoded separator falls.
func rejectSegment(segment string) error {
	if segment == "" {
		return nil
	}
	if segment == ".." {
		return errPathTraversal
	}
	decoded, err := url.PathUnescape(segment)
	if err != nil || decoded == segment {
		return nil
	}
	for subsegment := range strings.SplitSeq(decoded, "/") {
		if err := rejectSegment(subsegment); err != nil {
			return err
		}
	}
	return nil
}

// GetRawClientFn is a function type that returns a RawClient instance.
type GetRawClientFn func(context.Context) (*Client, error)

// Client is a client for interacting with the GitHub raw content API.
type Client struct {
	url    *url.URL
	client *gogithub.Client
}

// NewClient creates a new instance of the raw API Client with the provided GitHub client and provided URL.
func NewClient(client *gogithub.Client, rawURL *url.URL) (*Client, error) {
	newClient, err := gogithub.NewClient(
		gogithub.WithHTTPClient(client.Client()),
		gogithub.WithEnterpriseURLs(rawURL.String(), rawURL.String()),
	)
	if err != nil {
		return nil, err
	}
	return &Client{client: newClient, url: rawURL}, nil
}

func (c *Client) newRequest(ctx context.Context, method string, urlStr string, body any, opts ...gogithub.RequestOption) (*http.Request, error) {
	return c.client.NewRequest(ctx, method, urlStr, body, opts...)
}

func (c *Client) refURL(owner, repo, ref, path string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	if err := rejectPathTraversal(owner, repo, ref, path); err != nil {
		return "", err
	}
	return c.url.JoinPath(escapePathSegments(owner, repo, ref, path)...).String(), nil
}

func (c *Client) URLFromOpts(opts *ContentOpts, owner, repo, path string) (string, error) {
	if opts == nil {
		opts = &ContentOpts{}
	}
	if opts.SHA != "" {
		return c.commitURL(owner, repo, opts.SHA, path)
	}
	return c.refURL(owner, repo, opts.Ref, path)
}

// BlobURL returns the URL for a blob in the raw content API.
func (c *Client) commitURL(owner, repo, sha, path string) (string, error) {
	if err := rejectPathTraversal(owner, repo, sha, path); err != nil {
		return "", err
	}
	return c.url.JoinPath(escapePathSegments(owner, repo, sha, path)...).String(), nil
}

// escapePathSegments percent-encodes every "/"-separated segment of the given
// components. The components are decoded text (for example a file path taken
// from a repo:// resource URI), but url.URL.JoinPath treats its arguments as
// already escaped: a literal "%41" would be decoded again (fetching "A"), and
// an invalid escape such as "100%.txt" makes JoinPath silently fall back to
// the base URL. Escaping each segment keeps the file name exactly as given
// while preserving "/" as the path separator.
func escapePathSegments(components ...string) []string {
	escaped := make([]string, 0, len(components))
	for _, component := range components {
		segments := strings.Split(component, "/")
		for i, segment := range segments {
			segments[i] = url.PathEscape(segment)
		}
		escaped = append(escaped, strings.Join(segments, "/"))
	}
	return escaped
}

type ContentOpts struct {
	Ref string
	SHA string
}

// GetRawContent fetches the raw content of a file from a GitHub repository.
func (c *Client) GetRawContent(ctx context.Context, owner, repo, path string, opts *ContentOpts) (*http.Response, error) {
	rawURL, err := c.URLFromOpts(opts, owner, repo, path)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}

	return c.client.Client().Do(req)
}
