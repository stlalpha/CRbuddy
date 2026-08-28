package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/stlalpha/CRbuddy/internal/ghclient"
)

func TestTruncate(t *testing.T) {
	cases := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"shorter than max", "hi", 5, "hi"},
		{"exact length", "hello", 5, "hello"},
		{"longer than max", "hello world", 5, "hell…"},
		{"max of 1", "hello", 1, "…"},
		{"max of 0", "hello", 0, "…"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := truncate(c.s, c.max); got != c.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", c.s, c.max, got, c.want)
			}
		})
	}
}

func TestFormatUpdated(t *testing.T) {
	if got := formatUpdated(time.Time{}); got != "" {
		t.Errorf("formatUpdated(zero) = %q, want empty", got)
	}
	if got := formatUpdated(time.Now().Add(-30 * time.Second)); got != "just now" {
		t.Errorf("formatUpdated(30s ago) = %q, want %q", got, "just now")
	}
	if got := formatUpdated(time.Now().Add(-5 * time.Minute)); got != "5m ago" {
		t.Errorf("formatUpdated(5m ago) = %q, want %q", got, "5m ago")
	}
	if got := formatUpdated(time.Now().Add(-3 * time.Hour)); got != "3h ago" {
		t.Errorf("formatUpdated(3h ago) = %q, want %q", got, "3h ago")
	}
	if got := formatUpdated(time.Now().Add(-2 * 24 * time.Hour)); got != "2d ago" {
		t.Errorf("formatUpdated(2d ago) = %q, want %q", got, "2d ago")
	}
	old := time.Now().Add(-10 * 24 * time.Hour)
	if got, want := formatUpdated(old), old.Format("2006-01-02"); got != want {
		t.Errorf("formatUpdated(10d ago) = %q, want %q", got, want)
	}
}

func TestTitleColWidth(t *testing.T) {
	reserved := cursorColWidth + numColWidth + authorColWidth + updatedColWidth + statusReserve + colGap*4
	if got, want := titleColWidth(reserved+50), 50; got != want {
		t.Errorf("titleColWidth(reserved+50) = %d, want %d", got, want)
	}
	if got := titleColWidth(reserved - 100); got != 10 {
		t.Errorf("titleColWidth(too narrow) = %d, want floor of 10", got)
	}
}

func TestThreadCommentColWidth(t *testing.T) {
	reserved := threadCursorColWidth + statusColWidth + authorColWidth + pathColWidth + colGap*4
	if got, want := threadCommentColWidth(reserved+50), 50; got != want {
		t.Errorf("threadCommentColWidth(reserved+50) = %d, want %d", got, want)
	}
	if got := threadCommentColWidth(reserved - 100); got != 15 {
		t.Errorf("threadCommentColWidth(too narrow) = %d, want floor of 15", got)
	}
}

func TestReviewerLabel(t *testing.T) {
	cases := []struct {
		botLogin string
		want     string
	}{
		{"coderabbitai", "CodeRabbit"},
		{"CodeRabbitAI", "CodeRabbit"},
		{"", "Reviews"},
		{"alice", "alice"},
	}
	for _, c := range cases {
		if got := reviewerLabel(c.botLogin); got != c.want {
			t.Errorf("reviewerLabel(%q) = %q, want %q", c.botLogin, got, c.want)
		}
	}
}

func TestRenderThreadTable_ShowsAuthorAndHandlesEmpty(t *testing.T) {
	pr := ghclient.PR{Number: 1, Title: "t"}

	empty := renderThreadTable(pr, nil, 0, "Reviews", 120)
	if !strings.Contains(empty, "no Reviews threads on this PR") {
		t.Errorf("empty table = %q, want it to mention no Reviews threads", empty)
	}

	threads := []ghclient.Thread{
		{IsResolved: true, AuthorLogin: "alice", Path: "a.go", Body: "looks good"},
		{IsResolved: false, AuthorLogin: "coderabbitai", Path: "b.go", Body: "fix this"},
	}
	out := renderThreadTable(pr, threads, 0, "Reviews", 120)
	if !strings.Contains(out, "alice") {
		t.Errorf("table output missing author %q:\n%s", "alice", out)
	}
	if !strings.Contains(out, "coderabbitai") {
		t.Errorf("table output missing author %q:\n%s", "coderabbitai", out)
	}
}
