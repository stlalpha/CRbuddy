package ghclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stlalpha/prpal/internal/ghrepo"
)

// threadsPage builds one page of the threadsQuery response JSON.
type threadNode struct {
	isResolved bool
	isOutdated bool
	path       string
	// authorLogin == "" and hasComment == false together simulate a thread
	// with no comments at all; authorLogin == "" with hasComment == true
	// simulates a comment whose author is present but has an empty login.
	hasComment  bool
	hasAuthor   bool
	authorLogin string
	url         string
	body        string
}

func threadsPage(nodes []threadNode, hasNextPage bool, endCursor string) string {
	var nodesJSON []string
	for _, n := range nodes {
		commentsJSON := "[]"
		if n.hasComment {
			authorJSON := "null"
			if n.hasAuthor {
				authorJSON = fmt.Sprintf(`{"login":%q}`, n.authorLogin)
			}
			commentsJSON = fmt.Sprintf(`[{"author":%s,"url":%q,"body":%q}]`, authorJSON, n.url, n.body)
		}
		nodesJSON = append(nodesJSON, fmt.Sprintf(
			`{"isResolved":%v,"isOutdated":%v,"path":%q,"comments":{"nodes":%s}}`,
			n.isResolved, n.isOutdated, n.path, commentsJSON,
		))
	}
	return fmt.Sprintf(`{
		"data": {
			"repository": {
				"pullRequest": {
					"reviewThreads": {
						"pageInfo": {"hasNextPage": %v, "endCursor": %q},
						"nodes": [%s]
					}
				}
			}
		}
	}`, hasNextPage, endCursor, strings.Join(nodesJSON, ","))
}

func TestReviewThreads_BotMatching(t *testing.T) {
	nodes := []threadNode{
		{isResolved: true, path: "a.go", hasComment: true, hasAuthor: true, authorLogin: "coderabbitai", url: "u1", body: "b1"},
		{isResolved: false, path: "b.go", hasComment: true, hasAuthor: true, authorLogin: "coderabbitai[bot]", url: "u2", body: "b2"},
		{isResolved: false, path: "c.go", hasComment: true, hasAuthor: true, authorLogin: "CodeRabbitAI", url: "u3", body: "b3"},
		{isResolved: false, path: "d.go", hasComment: true, hasAuthor: true, authorLogin: "coderabbitai-extra", url: "u4", body: "b4"},
		{isResolved: false, path: "e.go", hasComment: true, hasAuthor: true, authorLogin: "someoneelse", url: "u5", body: "b5"},
	}
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(threadsPage(nodes, false, "")), nil
		},
	}}

	result, err := c.ReviewThreads(context.Background(), ghrepo.Repo{Owner: "o", Name: "r"}, 1, "coderabbitai")
	if err != nil {
		t.Fatalf("ReviewThreads() error = %v", err)
	}
	if len(result.Threads) != 3 {
		t.Fatalf("len(Threads) = %d, want 3 (exact login, [bot] suffix, case-insensitive)", len(result.Threads))
	}
	for _, th := range result.Threads {
		if th.Path == "d.go" || th.Path == "e.go" {
			t.Errorf("thread on %s should not have matched the bot login", th.Path)
		}
	}
	if result.Threads[0].Body != "b1" || result.Threads[0].URL != "u1" || !result.Threads[0].IsResolved {
		t.Errorf("Threads[0] = %+v, fields don't match input", result.Threads[0])
	}
}

func TestReviewThreads_AllReviewersMode(t *testing.T) {
	nodes := []threadNode{
		{path: "a.go", hasComment: true, hasAuthor: true, authorLogin: "coderabbitai", url: "u1", body: "b1"},
		{path: "b.go", hasComment: true, hasAuthor: true, authorLogin: "alice", url: "u2", body: "b2"},
		{path: "c.go", hasComment: true, hasAuthor: true, authorLogin: "bob", url: "u3", body: "b3"},
		{path: "no-comments.go", hasComment: false},
		{path: "nil-author.go", hasComment: true, hasAuthor: false},
	}
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(threadsPage(nodes, false, "")), nil
		},
	}}

	// Empty botLogin means "every reviewer" — no author filtering.
	result, err := c.ReviewThreads(context.Background(), ghrepo.Repo{Owner: "o", Name: "r"}, 1, "")
	if err != nil {
		t.Fatalf("ReviewThreads() error = %v", err)
	}
	if len(result.Threads) != 3 {
		t.Fatalf("len(Threads) = %d, want 3 (coderabbitai, alice, bob all included)", len(result.Threads))
	}
	gotAuthors := map[string]bool{}
	for _, th := range result.Threads {
		gotAuthors[th.AuthorLogin] = true
	}
	for _, want := range []string{"coderabbitai", "alice", "bob"} {
		if !gotAuthors[want] {
			t.Errorf("Threads missing author %q, got %+v", want, result.Threads)
		}
	}
	// Threads with no determinable author are still ambiguous, even in "all" mode.
	if result.Ambiguous != 2 {
		t.Errorf("Ambiguous = %d, want 2", result.Ambiguous)
	}
}

func TestReviewThreads_Ambiguous(t *testing.T) {
	nodes := []threadNode{
		{path: "no-comments.go", hasComment: false},
		{path: "nil-author.go", hasComment: true, hasAuthor: false},
		{path: "empty-login.go", hasComment: true, hasAuthor: true, authorLogin: ""},
		{path: "matched.go", hasComment: true, hasAuthor: true, authorLogin: "coderabbitai"},
	}
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(threadsPage(nodes, false, "")), nil
		},
	}}

	result, err := c.ReviewThreads(context.Background(), ghrepo.Repo{Owner: "o", Name: "r"}, 1, "coderabbitai")
	if err != nil {
		t.Fatalf("ReviewThreads() error = %v", err)
	}
	if result.Ambiguous != 3 {
		t.Errorf("Ambiguous = %d, want 3", result.Ambiguous)
	}
	if len(result.Threads) != 1 {
		t.Errorf("len(Threads) = %d, want 1", len(result.Threads))
	}
}

func TestReviewThreads_Pagination(t *testing.T) {
	page1 := []threadNode{{path: "p1.go", hasComment: true, hasAuthor: true, authorLogin: "coderabbitai"}}
	page2 := []threadNode{{path: "p2.go", hasComment: true, hasAuthor: true, authorLogin: "coderabbitai"}}

	var calls [][]string
	call := 0
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			calls = append(calls, args)
			call++
			if call == 1 {
				return []byte(threadsPage(page1, true, "cursor-1")), nil
			}
			return []byte(threadsPage(page2, false, "")), nil
		},
	}}

	result, err := c.ReviewThreads(context.Background(), ghrepo.Repo{Owner: "o", Name: "r"}, 1, "coderabbitai")
	if err != nil {
		t.Fatalf("ReviewThreads() error = %v", err)
	}
	if len(result.Threads) != 2 {
		t.Fatalf("len(Threads) = %d, want 2 (across both pages)", len(result.Threads))
	}
	if call != 2 {
		t.Fatalf("gh was called %d times, want 2", call)
	}
	if strings.Join(calls[0], " ") != "" && strings.Contains(strings.Join(calls[0], " "), "endCursor=") {
		t.Error("first call should not include an endCursor argument")
	}
	if !strings.Contains(strings.Join(calls[1], " "), "endCursor=cursor-1") {
		t.Error("second call should include the endCursor from page 1")
	}
}

func TestReviewThreads_TruncatesAtPageCap(t *testing.T) {
	calls := 0
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			calls++
			// Always claim there's another page, to force the hard cap.
			return []byte(threadsPage(nil, true, "next")), nil
		},
	}}

	result, err := c.ReviewThreads(context.Background(), ghrepo.Repo{Owner: "o", Name: "r"}, 1, "coderabbitai")
	if err != nil {
		t.Fatalf("ReviewThreads() error = %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated = false, want true after hitting the page cap")
	}
	if calls != maxPages {
		t.Errorf("gh was called %d times, want exactly maxPages (%d)", calls, maxPages)
	}
}

func TestReviewThreads_RunError(t *testing.T) {
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, &RunError{Cmd: "gh api graphql", Stderr: "boom", ExitCode: 1, Err: errors.New("exit status 1")}
		},
	}}
	_, err := c.ReviewThreads(context.Background(), ghrepo.Repo{Owner: "o", Name: "r"}, 42, "coderabbitai")
	if err == nil {
		t.Fatal("ReviewThreads() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "o/r#42") {
		t.Errorf("error = %q, want it to mention o/r#42", err.Error())
	}
}

func TestReviewThreads_PRNotFound(t *testing.T) {
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(`{"data":{"repository":{"pullRequest":null}}}`), nil
		},
	}}
	_, err := c.ReviewThreads(context.Background(), ghrepo.Repo{Owner: "o", Name: "r"}, 999, "coderabbitai")
	if err == nil {
		t.Fatal("ReviewThreads() error = nil, want an error for a missing PR")
	}
}

func TestReviewThreads_BadJSON(t *testing.T) {
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("not json"), nil
		},
	}}
	if _, err := c.ReviewThreads(context.Background(), ghrepo.Repo{Owner: "o", Name: "r"}, 1, "coderabbitai"); err == nil {
		t.Fatal("ReviewThreads() error = nil, want a parse error")
	}
}
