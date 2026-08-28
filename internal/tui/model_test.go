package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/stlalpha/prpal/internal/ghclient"
	"github.com/stlalpha/prpal/internal/tally"
)

// keyMsg builds a tea.KeyMsg for the given key name ("up", "down", "enter",
// "esc", "ctrl+c", or a single rune like "j", "k", "o", "r", "q").
func keyMsg(name string) tea.KeyMsg {
	switch name {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
	}
}

func update(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	res, _ := m.Update(msg)
	mm, ok := res.(Model)
	if !ok {
		t.Fatalf("Update() returned %T, want Model", res)
	}
	return mm
}

func TestUpdate_Quit(t *testing.T) {
	cancelled := false
	m := Model{state: stateReady, keys: newKeyMap(), cancel: func() { cancelled = true }}
	_, cmd := m.Update(keyMsg("q"))
	if !cancelled {
		t.Error("cancel was not called on quit")
	}
	if cmd == nil {
		t.Error("Update() on quit returned a nil cmd, want tea.Quit")
	}
}

func TestUpdate_ListNavigation(t *testing.T) {
	rows := []prRow{{pr: ghclient.PR{Number: 1}}, {pr: ghclient.PR{Number: 2}}, {pr: ghclient.PR{Number: 3}}}
	m := Model{state: stateReady, keys: newKeyMap(), rows: rows}

	m = update(t, m, keyMsg("down"))
	if m.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", m.cursor)
	}
	m = update(t, m, keyMsg("j"))
	if m.cursor != 2 {
		t.Fatalf("cursor after j = %d, want 2", m.cursor)
	}
	m = update(t, m, keyMsg("down")) // already at the last row
	if m.cursor != 2 {
		t.Fatalf("cursor past the last row = %d, want clamped to 2", m.cursor)
	}
	m = update(t, m, keyMsg("k"))
	if m.cursor != 1 {
		t.Fatalf("cursor after k = %d, want 1", m.cursor)
	}
	m.cursor = 0
	m = update(t, m, keyMsg("up")) // already at the first row
	if m.cursor != 0 {
		t.Fatalf("cursor before the first row = %d, want clamped to 0", m.cursor)
	}
}

func TestUpdate_SelectRequiresLoadedRow(t *testing.T) {
	m := Model{state: stateReady, keys: newKeyMap(), rows: []prRow{{pr: ghclient.PR{Number: 1}, loaded: false}}}
	m = update(t, m, keyMsg("enter"))
	if m.detail {
		t.Error("enter on an unloaded row should not open the detail view")
	}
}

func TestUpdate_SelectOpensDetail(t *testing.T) {
	m := Model{
		state: stateReady, keys: newKeyMap(),
		rows:         []prRow{{pr: ghclient.PR{Number: 1}, loaded: true, threads: []ghclient.Thread{{URL: "u"}}}},
		threadCursor: 5, notice: "stale",
	}
	m = update(t, m, keyMsg("enter"))
	if !m.detail {
		t.Fatal("enter on a loaded row should open the detail view")
	}
	if m.threadCursor != 0 {
		t.Errorf("threadCursor = %d, want reset to 0", m.threadCursor)
	}
	if m.notice != "" {
		t.Errorf("notice = %q, want cleared", m.notice)
	}
}

func TestUpdate_DetailBackAndSelectClose(t *testing.T) {
	for _, key := range []string{"esc", "enter"} {
		m := Model{state: stateReady, detail: true, keys: newKeyMap(), rows: []prRow{{}}, notice: "x"}
		m = update(t, m, keyMsg(key))
		if m.detail {
			t.Errorf("%q should close the detail view", key)
		}
		if m.notice != "" {
			t.Errorf("%q should clear notice, got %q", key, m.notice)
		}
	}
}

func TestUpdate_DetailThreadCursorBounds(t *testing.T) {
	threads := []ghclient.Thread{{}, {}, {}}
	m := Model{state: stateReady, detail: true, keys: newKeyMap(), rows: []prRow{{threads: threads}}}

	m = update(t, m, keyMsg("down"))
	if m.threadCursor != 1 {
		t.Fatalf("threadCursor after down = %d, want 1", m.threadCursor)
	}
	m = update(t, m, keyMsg("down"))
	if m.threadCursor != 2 {
		t.Fatalf("threadCursor after down = %d, want 2", m.threadCursor)
	}
	m = update(t, m, keyMsg("down")) // already at the last thread
	if m.threadCursor != 2 {
		t.Fatalf("threadCursor past the last thread = %d, want clamped to 2", m.threadCursor)
	}
	m = update(t, m, keyMsg("up"))
	if m.threadCursor != 1 {
		t.Fatalf("threadCursor after up = %d, want 1", m.threadCursor)
	}
}

func TestUpdate_OpenNoURL(t *testing.T) {
	threads := []ghclient.Thread{{URL: ""}}
	m := Model{
		state: stateReady, detail: true, keys: newKeyMap(),
		rows: []prRow{{threads: threads}},
		openURL: func(string) error {
			t.Fatal("openURL should not be called for a thread with no URL")
			return nil
		},
	}
	m = update(t, m, keyMsg("o"))
	if m.notice != "this comment has no URL" {
		t.Errorf("notice = %q, want %q", m.notice, "this comment has no URL")
	}
}

func TestUpdate_OpenSuccessClearsNotice(t *testing.T) {
	threads := []ghclient.Thread{{URL: "https://example.com/1"}}
	var gotURL string
	m := Model{
		state: stateReady, detail: true, keys: newKeyMap(),
		rows: []prRow{{threads: threads}}, notice: "old",
		openURL: func(u string) error { gotURL = u; return nil },
	}
	m = update(t, m, keyMsg("o"))
	if gotURL != "https://example.com/1" {
		t.Errorf("openURL called with %q, want the thread's URL", gotURL)
	}
	if m.notice != "" {
		t.Errorf("notice = %q, want cleared on success", m.notice)
	}
}

func TestUpdate_OpenFailureSetsNotice(t *testing.T) {
	wantErr := errors.New("boom")
	threads := []ghclient.Thread{{URL: "https://example.com/1"}}
	m := Model{
		state: stateReady, detail: true, keys: newKeyMap(),
		rows:    []prRow{{threads: threads}},
		openURL: func(string) error { return wantErr },
	}
	m = update(t, m, keyMsg("o"))
	if m.notice != wantErr.Error() {
		t.Errorf("notice = %q, want %q", m.notice, wantErr.Error())
	}
}

func TestUpdate_PRsMsg_ClampsCursor(t *testing.T) {
	m := Model{state: stateReady, keys: newKeyMap(), gen: 3, cursor: 5}
	m = update(t, m, prsMsg{gen: 3, prs: []ghclient.PR{{Number: 1}, {Number: 2}}})
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want clamped to the last index (1)", m.cursor)
	}
}

func TestUpdate_PRsMsg_EmptyListClearsDetail(t *testing.T) {
	m := Model{state: stateReady, keys: newKeyMap(), gen: 1, cursor: 0, detail: true}
	m = update(t, m, prsMsg{gen: 1, prs: nil})
	if m.detail {
		t.Error("detail should be cleared when the row list becomes empty")
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestUpdate_PRsMsg_StaleGenIgnored(t *testing.T) {
	m := Model{state: stateReady, keys: newKeyMap(), gen: 2, cursor: 1}
	m = update(t, m, prsMsg{gen: 1, prs: []ghclient.PR{{Number: 99}}})
	if len(m.rows) != 0 {
		t.Error("a stale-generation prsMsg should be ignored")
	}
}

func TestAggregate(t *testing.T) {
	m := Model{rows: []prRow{
		{loaded: true, tally: tally.Tally{Total: 3, Resolved: 1, Open: 2}},
		{loaded: false, tally: tally.Tally{Total: 99, Resolved: 99, Open: 99}}, // not loaded, excluded
		{loaded: true, tally: tally.Tally{Total: 2, Resolved: 2, Open: 0}},
	}}
	got := m.aggregate()
	want := tally.Tally{Total: 5, Resolved: 3, Open: 2}
	if got != want {
		t.Errorf("aggregate() = %+v, want %+v", got, want)
	}
}

func TestPartial(t *testing.T) {
	cases := []struct {
		name string
		rows []prRow
		want bool
	}{
		{"all clean", []prRow{{loaded: true}, {loaded: true}}, false},
		{"one not loaded", []prRow{{loaded: true}, {loaded: false}}, true},
		{"one errored", []prRow{{loaded: true}, {loaded: true, err: errors.New("x")}}, true},
		{"one truncated", []prRow{{loaded: true, truncated: true}}, true},
		{"one ambiguous", []prRow{{loaded: true, ambiguous: 1}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Model{rows: c.rows}
			if got := m.partial(); got != c.want {
				t.Errorf("partial() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestRebuildRows_PreservesLoadedState(t *testing.T) {
	m := Model{rows: []prRow{
		{pr: ghclient.PR{Number: 1, Title: "old title"}, loaded: true, tally: tally.Tally{Total: 5}},
	}}
	rows := m.rebuildRows([]ghclient.PR{
		{Number: 1, Title: "new title"},
		{Number: 2, Title: "brand new"},
	})
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].pr.Title != "new title" {
		t.Errorf("rows[0].pr.Title = %q, want the refreshed title", rows[0].pr.Title)
	}
	if !rows[0].loaded || rows[0].tally.Total != 5 {
		t.Errorf("rows[0] = %+v, want loaded/tally preserved across refresh", rows[0])
	}
	if rows[1].loaded {
		t.Error("a brand-new PR should start unloaded")
	}
}
