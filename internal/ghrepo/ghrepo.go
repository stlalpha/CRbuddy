package ghrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
)

// Repo identifies a GitHub repository.
type Repo struct {
	Owner string
	Name  string
}

// Slug returns "owner/name".
func (r Repo) Slug() string {
	return r.Owner + "/" + r.Name
}

// Sentinel errors — callers match with errors.Is.
var (
	ErrNotGitRepo      = errors.New("not inside a git repository")
	ErrNoGitHubRemote  = errors.New("no GitHub remote found")
	ErrGitNotInstalled = errors.New("git is not installed")
)

// scpLikeRE matches the scp-like syntax: user@host:path (no scheme).
var scpLikeRE = regexp.MustCompile(`^[^@\s]+@([^:/\s]+):(.+)$`)

// Detect inspects the git repo containing dir and returns the GitHub repo of its remote.
func Detect(ctx context.Context, dir string) (Repo, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return Repo{}, ErrGitNotInstalled
	}

	out, err := runGit(ctx, dir, "rev-parse", "--is-inside-work-tree")
	if err != nil || out != "true" {
		return Repo{}, ErrNotGitRepo
	}

	if originURL, err := runGit(ctx, dir, "remote", "get-url", "origin"); err == nil {
		if repo, ok := ParseGitHubURL(originURL); ok {
			return repo, nil
		}
	}

	remotesOut, err := runGit(ctx, dir, "remote")
	if err != nil {
		var ge *gitError
		if errors.As(err, &ge) {
			return Repo{}, fmt.Errorf("git: %w: %s", ge.err, ge.stderr)
		}
		return Repo{}, fmt.Errorf("git: %w: %s", err, "")
	}

	for _, name := range strings.Split(remotesOut, "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		remoteURL, err := runGit(ctx, dir, "remote", "get-url", name)
		if err != nil {
			continue
		}
		if repo, ok := ParseGitHubURL(remoteURL); ok {
			return repo, nil
		}
	}

	return Repo{}, ErrNoGitHubRemote
}

// ParseGitHubURL parses a git remote URL and reports whether it is a GitHub repo URL.
func ParseGitHubURL(raw string) (Repo, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Repo{}, false
	}

	var host, path string
	if m := scpLikeRE.FindStringSubmatch(s); m != nil && !strings.Contains(s, "://") {
		host = m[1]
		path = m[2]
	} else {
		u, err := url.Parse(s)
		if err != nil {
			return Repo{}, false
		}
		switch u.Scheme {
		case "ssh", "https", "http", "git":
		default:
			return Repo{}, false
		}
		host = u.Host
		path = u.Path
	}

	if !strings.EqualFold(host, "github.com") {
		return Repo{}, false
	}

	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(strings.TrimSpace(path), "/")
	path = strings.TrimSuffix(path, ".git")

	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Repo{}, false
	}

	return Repo{Owner: parts[0], Name: parts[1]}, true
}

// gitError carries the underlying exec error alongside trimmed stderr.
type gitError struct {
	err    error
	stderr string
}

func (e *gitError) Error() string {
	return e.err.Error()
}

func (e *gitError) Unwrap() error {
	return e.err
}

// runGit runs `git -C dir <args...>` and returns trimmed stdout. On failure it
// returns a *gitError wrapping the exec error with trimmed stderr.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return strings.TrimSpace(stdout.String()), &gitError{err: err, stderr: strings.TrimSpace(stderr.String())}
	}
	return strings.TrimSpace(stdout.String()), nil
}
