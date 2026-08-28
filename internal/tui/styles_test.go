package tui

import (
	"testing"
	"time"
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
	reserved := threadCursorColWidth + statusColWidth + pathColWidth + colGap*3
	if got, want := threadCommentColWidth(reserved+50), 50; got != want {
		t.Errorf("threadCommentColWidth(reserved+50) = %d, want %d", got, want)
	}
	if got := threadCommentColWidth(reserved - 100); got != 15 {
		t.Errorf("threadCommentColWidth(too narrow) = %d, want floor of 15", got)
	}
}
