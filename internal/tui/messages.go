package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/stlalpha/prpal/internal/ghclient"
	"github.com/stlalpha/prpal/internal/ghrepo"
)

// preflightMsg carries the result of startup checks. Exactly one of repo/err is meaningful.
type preflightMsg struct {
	repo ghrepo.Repo
	err  error
}

// prsMsg carries the open-PR list for refresh generation gen.
type prsMsg struct {
	gen int
	prs []ghclient.PR
	err error
}

// threadsMsg carries one PR's CodeRabbit threads for refresh generation gen.
type threadsMsg struct {
	gen       int
	number    int
	threads   []ghclient.Thread
	ambiguous int  // threads whose first-comment author couldn't be determined
	truncated bool // true if pagination hit the hard cap before finishing
	err       error
}

// tickMsg fires the auto-refresh interval.
type tickMsg time.Time

const cmdTimeout = 30 * time.Second

// preflightCmd runs repo detection then auth check; the first failure wins.
// ctx is the program's lifetime context (cancelled on quit) — preflightCmd
// derives its own timeout from it so the gh subprocess it may spawn never
// outlives the program.
func preflightCmd(ctx context.Context, client *ghclient.Client, dir string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, cmdTimeout)
		defer cancel()

		repo, err := ghrepo.Detect(ctx, dir)
		if err != nil {
			return preflightMsg{err: err}
		}
		if err := client.CheckAuth(ctx); err != nil {
			return preflightMsg{err: err}
		}
		return preflightMsg{repo: repo}
	}
}

// fetchPRsCmd fetches the open PR list for the given repo. ctx should be the
// current refresh generation's context, so a superseded generation's fetch
// (or one still in flight at quit) is cancelled instead of running to
// completion detached.
func fetchPRsCmd(ctx context.Context, client *ghclient.Client, repo ghrepo.Repo, limit, gen int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, cmdTimeout)
		defer cancel()

		prs, err := client.ListOpenPRs(ctx, repo, limit)
		if err != nil {
			return prsMsg{gen: gen, err: err}
		}
		return prsMsg{gen: gen, prs: prs}
	}
}

// fetchThreadsCmd fetches a single PR's CodeRabbit review threads. It fetches
// with no overall deadline: ReviewThreads applies its own per-page timeout
// internally, so a PR spanning many pages isn't cut off by a single shared budget.
// ctx should be the current refresh generation's context, so all in-flight
// thread fetches from a superseded generation (or one still in flight at
// quit) are cancelled together instead of running to completion detached.
func fetchThreadsCmd(ctx context.Context, client *ghclient.Client, repo ghrepo.Repo, number int, botLogin string, gen int) tea.Cmd {
	return func() tea.Msg {
		result, err := client.ReviewThreads(ctx, repo, number, botLogin)
		if err != nil {
			return threadsMsg{gen: gen, number: number, err: err}
		}
		return threadsMsg{
			gen:       gen,
			number:    number,
			threads:   result.Threads,
			ambiguous: result.Ambiguous,
			truncated: result.Truncated,
		}
	}
}

// tickCmd schedules the next auto-refresh tick.
func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
