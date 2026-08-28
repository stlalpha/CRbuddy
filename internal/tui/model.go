package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/stlalpha/prpal/internal/config"
	"github.com/stlalpha/prpal/internal/ghclient"
	"github.com/stlalpha/prpal/internal/ghrepo"
	"github.com/stlalpha/prpal/internal/tally"
)

// state enumerates the top-level UI mode.
type state int

const (
	stateChecking state = iota // preflight running
	stateFatal                 // preflight failed — error screen, only quit works
	stateLoading               // first PR-list fetch in flight, nothing to show yet
	stateError                 // first PR-list fetch failed — distinct error screen, r retries
	stateReady                 // table rendered; refreshes update in place
)

// prRow is one PR plus its thread tally.
type prRow struct {
	pr        ghclient.PR
	tally     tally.Tally
	threads   []ghclient.Thread // raw CodeRabbit threads for this PR, for the detail table
	loaded    bool              // threads fetched at least once this session
	err       error             // last per-PR thread-fetch error; rendered inline on the row, cleared on next success
	ambiguous int               // threads whose bot ownership couldn't be determined; tally may be undercounting
	truncated bool              // pagination hit the hard cap before finishing; tally may be incomplete
}

// keyMap uses bubbles/key. Bindings: Quit = "q", "ctrl+c"; Refresh = "r";
// Up/Down move the row cursor (list mode) or the thread cursor (detail mode);
// Select opens detail for the selected PR; Back returns to the list;
// Open opens the selected thread's URL in the browser (detail mode only).
type keyMap struct {
	Quit    key.Binding
	Refresh key.Binding
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
	Back    key.Binding
	Open    key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c")),
		Refresh: key.NewBinding(key.WithKeys("r")),
		Up:      key.NewBinding(key.WithKeys("up", "k")),
		Down:    key.NewBinding(key.WithKeys("down", "j")),
		Select:  key.NewBinding(key.WithKeys("enter")),
		Back:    key.NewBinding(key.WithKeys("esc")),
		Open:    key.NewBinding(key.WithKeys("o")),
	}
}

// Model implements tea.Model. Value receiver throughout (Bubble Tea convention).
type Model struct {
	cfg     config.Config
	client  *ghclient.Client
	dir     string
	repo    ghrepo.Repo
	state   state
	fatal   error // set iff state == stateFatal
	rows    []prRow
	gen     int // refresh generation; incremented on every refresh kickoff
	pending int // outstanding threadsMsg count for current gen (drives "refreshing" indicator)
	lastErr error
	lastOK  time.Time
	spin    spinner.Model
	keys    keyMap
	width   int
	height  int

	cursor       int    // selected row index in the PR list
	detail       bool   // true when showing the thread detail table for rows[cursor]
	threadCursor int    // selected thread index within the detail table
	notice       string // transient message (e.g. a failed browser-open) shown in the detail view footer until the next action

	openURL func(string) error // opens a URL in the browser; overridable in tests

	ctx    context.Context    // program lifetime; cancelled on quit so in-flight gh subprocesses are killed
	cancel context.CancelFunc // cancels ctx

	genCtx    context.Context    // current refresh generation's fetch context, child of ctx
	genCancel context.CancelFunc // cancels genCtx; called at the start of every startRefresh to kill the prior generation's in-flight fetches
}

// New constructs the initial Model in stateChecking with a spinner.Dot spinner.
func New(cfg config.Config, client *ghclient.Client, dir string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		cfg:     cfg,
		client:  client,
		dir:     dir,
		state:   stateChecking,
		spin:    s,
		keys:    newKeyMap(),
		ctx:     ctx,
		cancel:  cancel,
		openURL: openURL,
	}
}

// Init returns tea.Batch(m.spin.Tick, preflightCmd(m.ctx, m.client, m.dir)).
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, preflightCmd(m.ctx, m.client, m.dir))
}

// startRefresh cancels the previous generation's in-flight fetches, bumps the
// refresh generation, and kicks off a fresh PR-list fetch. pending is set
// immediately so the refreshing indicator is active for the whole refresh,
// including the PR-list phase before any thread fetch has even started.
func (m Model) startRefresh() (Model, tea.Cmd) {
	if m.genCancel != nil {
		m.genCancel()
	}
	m.gen++
	m.genCtx, m.genCancel = context.WithCancel(m.ctx)
	m.pending = 1
	return m, fetchPRsCmd(m.genCtx, m.client, m.repo, m.cfg.PRLimit, m.gen)
}

// rebuildRows folds a fresh PR list into rows, preserving tally/loaded/err for
// PRs that were already present so counts don't flash to zero on refresh.
func (m Model) rebuildRows(prs []ghclient.PR) []prRow {
	existing := make(map[int]prRow, len(m.rows))
	for _, r := range m.rows {
		existing[r.pr.Number] = r
	}
	rows := make([]prRow, 0, len(prs))
	for _, pr := range prs {
		if old, ok := existing[pr.Number]; ok {
			old.pr = pr
			rows = append(rows, old)
			continue
		}
		rows = append(rows, prRow{pr: pr})
	}
	return rows
}

// aggregate sums tallies over rows that have been loaded at least once.
func (m Model) aggregate() tally.Tally {
	var agg tally.Tally
	for _, r := range m.rows {
		if r.loaded {
			agg = agg.Add(r.tally)
		}
	}
	return agg
}

// partial reports whether the aggregate tally may be undercounting: some row
// hasn't completed a thread fetch yet, errored on its last one, or came back
// truncated/ambiguous.
func (m Model) partial() bool {
	for _, r := range m.rows {
		if !r.loaded || r.err != nil || r.truncated || r.ambiguous > 0 {
			return true
		}
	}
	return false
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.Refresh) && (m.state == stateReady || m.state == stateLoading || m.state == stateError) {
			return m.startRefresh()
		}

		if m.state != stateReady {
			return m, nil
		}

		if m.detail {
			threads := m.rows[m.cursor].threads
			switch {
			case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Select):
				m.detail = false
				m.notice = ""
				return m, nil
			case key.Matches(msg, m.keys.Up):
				if m.threadCursor > 0 {
					m.threadCursor--
				}
				m.notice = ""
				return m, nil
			case key.Matches(msg, m.keys.Down):
				if m.threadCursor < len(threads)-1 {
					m.threadCursor++
				}
				m.notice = ""
				return m, nil
			case key.Matches(msg, m.keys.Open):
				m.notice = ""
				if m.threadCursor >= 0 && m.threadCursor < len(threads) {
					url := threads[m.threadCursor].URL
					if url == "" {
						m.notice = "this comment has no URL"
					} else if err := m.openURL(url); err != nil {
						m.notice = err.Error()
					}
				}
				return m, nil
			}
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Select):
			if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].loaded {
				m.detail = true
				m.threadCursor = 0
				m.notice = ""
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		if m.state == stateChecking || m.state == stateLoading || m.pending > 0 {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case preflightMsg:
		if msg.err != nil {
			m.state = stateFatal
			m.fatal = msg.err
			return m, nil
		}
		m.repo = msg.repo
		m.state = stateLoading
		newM, cmd := m.startRefresh()
		return newM, tea.Batch(cmd, tickCmd(m.cfg.RefreshInterval))

	case prsMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		if msg.err != nil {
			m.lastErr = msg.err
			m.pending = 0
			if m.state == stateLoading {
				m.state = stateError
			}
			return m, nil
		}
		m.lastErr = nil
		m.state = stateReady
		m.rows = m.rebuildRows(msg.prs)
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
			m.detail = false
		}
		m.pending = len(msg.prs)
		if m.pending == 0 {
			m.lastOK = time.Now()
			return m, nil
		}
		cmds := make([]tea.Cmd, 0, len(msg.prs))
		for _, pr := range msg.prs {
			cmds = append(cmds, fetchThreadsCmd(m.genCtx, m.client, m.repo, pr.Number, m.cfg.BotLogin, m.gen))
		}
		return m, tea.Batch(cmds...)

	case threadsMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		for i := range m.rows {
			if m.rows[i].pr.Number != msg.number {
				continue
			}
			if msg.err != nil {
				m.rows[i].err = msg.err
			} else {
				m.rows[i].tally = tally.Count(msg.threads)
				m.rows[i].threads = msg.threads
				m.rows[i].loaded = true
				m.rows[i].err = nil
				m.rows[i].ambiguous = msg.ambiguous
				m.rows[i].truncated = msg.truncated
			}
			break
		}
		if m.pending > 0 {
			m.pending--
		}
		if m.pending == 0 && m.lastErr == nil {
			clean := true
			for _, r := range m.rows {
				if r.err != nil {
					clean = false
					break
				}
			}
			if clean {
				m.lastOK = time.Now()
			}
		}
		return m, nil

	case tickMsg:
		newM, cmd := m.startRefresh()
		return newM, tea.Batch(cmd, tickCmd(m.cfg.RefreshInterval))
	}

	return m, nil
}

// fatalHint prepends a one-line human hint for well-known preflight failures.
func fatalHint(err error) string {
	switch {
	case errors.Is(err, ghrepo.ErrNotGitRepo):
		return "Run this inside a git repo with a GitHub remote."
	case errors.Is(err, ghrepo.ErrNoGitHubRemote):
		return "Add a GitHub remote to this repo (e.g. `git remote add origin ...`)."
	case errors.Is(err, ghclient.ErrGHNotInstalled):
		return "Install the GitHub CLI: https://cli.github.com"
	case errors.Is(err, ghclient.ErrGHNotAuthenticated):
		return "Run `gh auth login` to authenticate."
	default:
		return ""
	}
}

func (m Model) View() string {
	label := reviewerLabel(m.cfg.BotLogin)
	switch m.state {
	case stateChecking:
		return fmt.Sprintf("%s checking environment…", m.spin.View())

	case stateFatal:
		body := m.fatal.Error()
		if hint := fatalHint(m.fatal); hint != "" {
			body = hint + "\n\n" + body
		}
		return errBox.Render(body + "\n\npress q to quit")

	case stateLoading:
		header := renderHeader(m.repo, tally.Tally{}, false, m.lastOK, true, m.spin.View(), label, m.width)
		lines := []string{header, fmt.Sprintf("%s loading pull requests…", m.spin.View())}
		return strings.Join(lines, "\n")

	case stateError:
		spin := ""
		if m.pending > 0 {
			spin = m.spin.View()
		}
		header := renderHeader(m.repo, tally.Tally{}, false, m.lastOK, m.pending > 0, spin, label, m.width)
		body := m.lastErr.Error()
		if hint := fatalHint(m.lastErr); hint != "" {
			body = hint + "\n\n" + body
		}
		lines := []string{header, "", errBox.Render(body + "\n\npress r to retry · q to quit")}
		return strings.Join(lines, "\n")

	case stateReady:
		spin := ""
		if m.pending > 0 {
			spin = m.spin.View()
		}
		header := renderHeader(m.repo, m.aggregate(), m.partial(), m.lastOK, m.pending > 0, spin, label, m.width)

		if m.detail && m.cursor >= 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			lines := []string{header, "", renderThreadTable(row.pr, row.threads, m.threadCursor, label, m.width)}
			if m.notice != "" {
				lines = append(lines, errStyle.Render(m.notice))
			} else {
				lines = append(lines, dimStyle.Render("↑/k ↓/j select · o open in browser · esc/enter back · q quit"))
			}
			return strings.Join(lines, "\n")
		}

		lines := []string{header}
		for i, r := range m.rows {
			lines = append(lines, renderRow(r, i == m.cursor, label, m.width))
		}
		if m.lastErr != nil {
			lines = append(lines, errStyle.Render(m.lastErr.Error()))
		} else {
			lines = append(lines, dimStyle.Render(fmt.Sprintf("↑/k ↓/j select · enter view comments · r refresh · q quit · every %s", m.cfg.RefreshInterval)))
		}
		return strings.Join(lines, "\n")
	}

	return ""
}
