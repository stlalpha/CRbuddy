package ghclient

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stlalpha/CRbuddy/internal/ghrepo"
)

func TestListOpenPRs_Success(t *testing.T) {
	const raw = `[
		{"number":1,"title":"First","author":{"login":"alice"},"url":"https://github.com/o/r/pull/1","headRefName":"alice/first","isDraft":false,"updatedAt":"2026-08-01T00:00:00Z"},
		{"number":2,"title":"Second (no author)","author":null,"url":"https://github.com/o/r/pull/2","headRefName":"bob/second","isDraft":true,"updatedAt":"2026-08-02T00:00:00Z"}
	]`

	var gotArgs []string
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			gotArgs = args
			return []byte(raw), nil
		},
	}}

	prs, err := c.ListOpenPRs(context.Background(), ghrepo.Repo{Owner: "o", Name: "r"}, 25)
	if err != nil {
		t.Fatalf("ListOpenPRs() error = %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("len(prs) = %d, want 2", len(prs))
	}

	want0 := PR{
		Number: 1, Title: "First", Author: "alice",
		URL: "https://github.com/o/r/pull/1", HeadRef: "alice/first", IsDraft: false,
		UpdatedAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}
	if prs[0] != want0 {
		t.Errorf("prs[0] = %+v, want %+v", prs[0], want0)
	}
	if prs[1].Author != "" {
		t.Errorf("prs[1].Author = %q, want empty (null author)", prs[1].Author)
	}
	if !prs[1].IsDraft {
		t.Error("prs[1].IsDraft = false, want true")
	}

	argStr := strings.Join(gotArgs, " ")
	if !strings.Contains(argStr, "--repo o/r") {
		t.Errorf("args = %q, want it to contain --repo o/r", argStr)
	}
	if !strings.Contains(argStr, "--limit 25") {
		t.Errorf("args = %q, want it to contain --limit 25", argStr)
	}
}

func TestListOpenPRs_RunError(t *testing.T) {
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, &RunError{Cmd: "gh pr list", Stderr: "not found", ExitCode: 1, Err: errors.New("exit status 1")}
		},
	}}
	_, err := c.ListOpenPRs(context.Background(), ghrepo.Repo{Owner: "o", Name: "r"}, 10)
	if err == nil {
		t.Fatal("ListOpenPRs() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "o/r") {
		t.Errorf("error = %q, want it to mention the repo slug", err.Error())
	}
}

func TestListOpenPRs_BadJSON(t *testing.T) {
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("not json"), nil
		},
	}}
	if _, err := c.ListOpenPRs(context.Background(), ghrepo.Repo{Owner: "o", Name: "r"}, 10); err == nil {
		t.Fatal("ListOpenPRs() error = nil, want a parse error")
	}
}
