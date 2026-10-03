package agentctl

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ownerPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)
	repoPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

func validRepo(repo string) bool {
	parts := strings.Split(repo, "/")
	return len(parts) == 2 && ownerPattern.MatchString(parts[0]) &&
		repoPattern.MatchString(parts[1]) && parts[1] != "." && parts[1] != ".."
}

func parseRemote(remote string) (string, error) {
	var repo string
	if path, ok := strings.CutPrefix(remote, "git@github.com:"); ok {
		repo = path
	} else {
		u, err := url.Parse(remote)
		if err != nil || u.Host != "github.com" || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
			return "", fmt.Errorf("origin must be a normal github.com HTTPS or SSH remote")
		}
		switch u.Scheme {
		case "https":
			if u.User != nil {
				return "", fmt.Errorf("credential-bearing HTTPS origins are not supported; use https://github.com/owner/repo.git")
			}
		case "ssh":
			if u.User == nil || u.User.String() != "git" {
				return "", fmt.Errorf("SSH origin must use git@github.com")
			}
		default:
			return "", fmt.Errorf("origin must use HTTPS or SSH on github.com")
		}
		repo = strings.TrimPrefix(u.Path, "/")
	}
	repo = strings.TrimSuffix(repo, ".git")
	if !validRepo(repo) {
		return "", fmt.Errorf("origin must identify exactly one GitHub owner/repo")
	}
	return repo, nil
}

func validateCheckout(ctx context.Context, path, expected string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if expected != "" && !validRepo(expected) {
		return "", fmt.Errorf("--repo must have the form owner/repo")
	}
	if path == "" {
		path = "."
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve --path: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("--path must be an existing local checkout directory")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git executable not found on PATH; install Git to validate the checkout: %w", err)
	}
	root, err := gitOutput(ctx, absolute, "rev-parse", "--show-toplevel")
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil || root == "" {
		return "", fmt.Errorf("selected directory is not a Git working tree; explicitly clone the repository first and pass --path")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve Git working tree root: %w", err)
	}
	selected, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve selected checkout: %w", err)
	}
	relative, err := filepath.Rel(root, selected)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("git working tree root is outside the selected directory; select a directory inside the intended checkout")
	}
	remote, err := gitOutput(ctx, root, "config", "--get-all", "remote.origin.url")
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", fmt.Errorf("checkout has no single origin URL; configure a github.com HTTPS or SSH origin")
	}
	actual, err := parseRemote(remote)
	if err != nil {
		return "", err
	}
	if expected != "" && !strings.EqualFold(expected, actual) {
		return "", fmt.Errorf("checkout origin is %s, not --repo %s; select the correct checkout with --path", actual, expected)
	}
	return root, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = checkoutEnvironment()
	output, err := cmd.Output()
	return strings.TrimSuffix(strings.TrimSuffix(string(output), "\n"), "\r"), err
}

func checkoutEnvironment() []string {
	// Validation and agent tools must agree on the selected repository.
	var env []string
	for _, variable := range os.Environ() {
		key, _, _ := strings.Cut(variable, "=")
		if !strings.HasPrefix(key, "GIT_") {
			env = append(env, variable)
		}
	}
	return env
}
