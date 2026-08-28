package ghrepo

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestParseGitHubURL(t *testing.T) {
	cases := []struct {
		name   string
		raw    string
		want   Repo
		wantOK bool
	}{
		{"scp-like ssh", "git@github.com:owner/name.git", Repo{"owner", "name"}, true},
		{"ssh url", "ssh://git@github.com/owner/name.git", Repo{"owner", "name"}, true},
		{"https with .git", "https://github.com/owner/name.git", Repo{"owner", "name"}, true},
		{"https without .git", "https://github.com/owner/name", Repo{"owner", "name"}, true},
		{"http", "http://github.com/owner/name", Repo{"owner", "name"}, true},
		{"git protocol", "git://github.com/owner/name.git", Repo{"owner", "name"}, true},
		{"trailing slash", "https://github.com/owner/name/", Repo{"owner", "name"}, true},
		{"leading/trailing whitespace", "  https://github.com/owner/name.git  ", Repo{"owner", "name"}, true},
		{"case-insensitive host", "https://GitHub.com/owner/name", Repo{"owner", "name"}, true},
		{"non-github host", "git@gitlab.com:owner/name.git", Repo{}, false},
		{"empty", "", Repo{}, false},
		{"malformed", "not a url", Repo{}, false},
		{"missing repo segment", "https://github.com/owner", Repo{}, false},
		{"extra path segment", "https://github.com/owner/name/extra", Repo{}, false},
		{"unsupported scheme", "ftp://github.com/owner/name", Repo{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ParseGitHubURL(c.raw)
			if ok != c.wantOK {
				t.Fatalf("ParseGitHubURL(%q) ok = %v, want %v", c.raw, ok, c.wantOK)
			}
			if ok && got != c.want {
				t.Errorf("ParseGitHubURL(%q) = %+v, want %+v", c.raw, got, c.want)
			}
		})
	}
}

// initGitRepo creates a bare-minimum git repo in dir with the given remotes
// (name -> URL). Skips the test if git isn't available in this environment.
func initGitRepo(t *testing.T, dir string, remotes map[string]string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	for name, url := range remotes {
		run("remote", "add", name, url)
	}
}

func TestDetect_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := Detect(context.Background(), dir)
	if !errors.Is(err, ErrNotGitRepo) {
		t.Fatalf("Detect() error = %v, want ErrNotGitRepo", err)
	}
}

func TestDetect_NoGitHubRemote(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, nil)
	_, err := Detect(context.Background(), dir)
	if !errors.Is(err, ErrNoGitHubRemote) {
		t.Fatalf("Detect() error = %v, want ErrNoGitHubRemote", err)
	}
}

func TestDetect_OriginIsGitHub(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, map[string]string{"origin": "git@github.com:owner/name.git"})
	repo, err := Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if repo.Slug() != "owner/name" {
		t.Errorf("Detect() = %+v, want owner/name", repo)
	}
}

// Origin exists but isn't GitHub; Detect must fall back to scanning other
// remotes rather than returning ErrNoGitHubRemote immediately.
func TestDetect_OriginNonGitHubFallsBackToOtherRemote(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, map[string]string{
		"origin":   "git@gitlab.com:owner/wrong.git",
		"upstream": "git@github.com:owner/name.git",
	})
	repo, err := Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if repo.Slug() != "owner/name" {
		t.Errorf("Detect() = %+v, want owner/name", repo)
	}
}

func TestDetect_GitNotInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Detect(context.Background(), t.TempDir())
	if !errors.Is(err, ErrGitNotInstalled) {
		t.Fatalf("Detect() error = %v, want ErrGitNotInstalled", err)
	}
}

func TestDetect_ContextTimeout(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir, map[string]string{"origin": "git@github.com:owner/name.git"})

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // ensure the deadline has passed

	if _, err := Detect(ctx, dir); err == nil {
		t.Fatal("Detect() with an expired context: want an error, got nil")
	}
}
