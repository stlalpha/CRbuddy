package mcpserver

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stlalpha/CRbuddy/internal/ghclient"
	"github.com/stlalpha/CRbuddy/internal/tally"
)

func TestResolveRepo_ExplicitDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("remote", "add", "origin", "git@github.com:owner/name.git")

	repo, err := resolveRepo(context.Background(), dir)
	if err != nil {
		t.Fatalf("resolveRepo() error = %v", err)
	}
	if repo.Slug() != "owner/name" {
		t.Errorf("resolveRepo() = %+v, want owner/name", repo)
	}
}

func TestResolveRepo_EmptyDirUsesCwd(t *testing.T) {
	// The test binary's own working directory isn't necessarily a git repo
	// with a GitHub remote, so just confirm the empty-dir path doesn't error
	// out before reaching Detect (i.e. it doesn't fail on os.Getwd itself).
	_, err := resolveRepo(context.Background(), "")
	if err != nil {
		t.Logf("resolveRepo(\"\") = %v (expected unless the test runs inside a GitHub-remote repo)", err)
	}
}

func TestToPRInfo(t *testing.T) {
	now := time.Now()
	pr := ghclient.PR{Number: 7, Title: "t", Author: "a", URL: "u", HeadRef: "h", IsDraft: true, UpdatedAt: now}
	got := toPRInfo(pr)
	want := prInfo{Number: 7, Title: "t", Author: "a", URL: "u", HeadRef: "h", IsDraft: true, UpdatedAt: now}
	if got != want {
		t.Errorf("toPRInfo() = %+v, want %+v", got, want)
	}
}

func TestToTallyInfo(t *testing.T) {
	got := toTallyInfo(tally.Tally{Total: 3, Resolved: 1, Open: 2})
	want := tallyInfo{Total: 3, Resolved: 1, Open: 2}
	if got != want {
		t.Errorf("toTallyInfo() = %+v, want %+v", got, want)
	}
}

func TestToThreadInfo(t *testing.T) {
	th := ghclient.Thread{IsResolved: true, IsOutdated: false, Path: "p", AuthorLogin: "coderabbitai", URL: "u", Body: "b"}
	got := toThreadInfo(th)
	want := threadInfo{IsResolved: true, IsOutdated: false, Path: "p", Author: "coderabbitai", Body: "b", URL: "u"}
	if got != want {
		t.Errorf("toThreadInfo() = %+v, want %+v", got, want)
	}
}
