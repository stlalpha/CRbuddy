// Package mcpserver exposes CodeRabbit Buddy's PR and review-tally data as
// MCP tools over stdio, so an agent can query a repo's review status without
// going through the interactive TUI. It is a read-only wrapper around
// internal/ghclient and internal/tally — no GitHub-fetching or tally logic
// is duplicated here.
package mcpserver

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stlalpha/CRbuddy/internal/config"
	"github.com/stlalpha/CRbuddy/internal/ghclient"
	"github.com/stlalpha/CRbuddy/internal/ghrepo"
	"github.com/stlalpha/CRbuddy/internal/tally"
)

// cmdTimeout bounds every gh invocation a tool call makes, matching the TUI's
// per-command budget.
const cmdTimeout = 30 * time.Second

// resolveRepo defaults dir to the process's working directory, then detects
// the GitHub repo the same way the TUI does at startup.
func resolveRepo(ctx context.Context, dir string) (ghrepo.Repo, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return ghrepo.Repo{}, fmt.Errorf("get working directory: %w", err)
		}
		dir = wd
	}
	return ghrepo.Detect(ctx, dir)
}

// prInfo is the JSON shape returned for one PR.
type prInfo struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Author    string    `json:"author"`
	URL       string    `json:"url"`
	HeadRef   string    `json:"head_ref"`
	IsDraft   bool      `json:"is_draft"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toPRInfo(pr ghclient.PR) prInfo {
	return prInfo{
		Number:    pr.Number,
		Title:     pr.Title,
		Author:    pr.Author,
		URL:       pr.URL,
		HeadRef:   pr.HeadRef,
		IsDraft:   pr.IsDraft,
		UpdatedAt: pr.UpdatedAt,
	}
}

// tallyInfo is the JSON shape for a resolved/open/total count.
type tallyInfo struct {
	Total    int `json:"total"`
	Resolved int `json:"resolved"`
	Open     int `json:"open"`
}

func toTallyInfo(t tally.Tally) tallyInfo {
	return tallyInfo{Total: t.Total, Resolved: t.Resolved, Open: t.Open}
}

// threadInfo is the JSON shape for one CodeRabbit review thread.
type threadInfo struct {
	IsResolved bool   `json:"is_resolved"`
	IsOutdated bool   `json:"is_outdated"`
	Path       string `json:"path"`
	Body       string `json:"body"`
	URL        string `json:"url"`
}

func toThreadInfo(t ghclient.Thread) threadInfo {
	return threadInfo{
		IsResolved: t.IsResolved,
		IsOutdated: t.IsOutdated,
		Path:       t.Path,
		Body:       t.Body,
		URL:        t.URL,
	}
}

// listOpenPRsIn is the input for the list_open_prs tool.
type listOpenPRsIn struct {
	Dir string `json:"dir,omitempty" jsonschema:"directory inside the git repo to inspect; defaults to the server's working directory"`
}

// listOpenPRsOut is the output for the list_open_prs tool.
type listOpenPRsOut struct {
	Repo string   `json:"repo"`
	PRs  []prInfo `json:"prs"`
}

// prReviewTallyOut is one PR's entry in the get_repo_review_tally tool's output.
type prReviewTallyOut struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Tally     tallyInfo `json:"tally"`
	Truncated bool      `json:"truncated,omitempty"`
	Ambiguous int       `json:"ambiguous,omitempty"`
}

// repoReviewTallyIn is the input for the get_repo_review_tally tool.
type repoReviewTallyIn struct {
	Dir string `json:"dir,omitempty" jsonschema:"directory inside the git repo to inspect; defaults to the server's working directory"`
}

// repoReviewTallyOut is the output for the get_repo_review_tally tool.
type repoReviewTallyOut struct {
	Repo      string             `json:"repo"`
	Aggregate tallyInfo          `json:"aggregate"`
	PRs       []prReviewTallyOut `json:"prs"`
}

// prReviewCommentsIn is the input for the get_pr_review_comments tool.
type prReviewCommentsIn struct {
	Dir      string `json:"dir,omitempty" jsonschema:"directory inside the git repo to inspect; defaults to the server's working directory"`
	PRNumber int    `json:"pr_number" jsonschema:"the pull request number to fetch CodeRabbit comments for"`
}

// prReviewCommentsOut is the output for the get_pr_review_comments tool.
type prReviewCommentsOut struct {
	Repo      string       `json:"repo"`
	PRNumber  int          `json:"pr_number"`
	Threads   []threadInfo `json:"threads"`
	Truncated bool         `json:"truncated,omitempty"`
	Ambiguous int          `json:"ambiguous,omitempty"`
}

// New builds an MCP server exposing client's PR/review data as read-only
// tools, using cfg for the default bot login and PR fetch limit.
func New(client *ghclient.Client, cfg config.Config) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "coderabbit-buddy", Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_open_prs",
		Description: "List open pull requests for the GitHub repo at the given directory (or the server's cwd if omitted).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listOpenPRsIn) (*mcp.CallToolResult, listOpenPRsOut, error) {
		ctx, cancel := context.WithTimeout(ctx, cmdTimeout)
		defer cancel()

		repo, err := resolveRepo(ctx, in.Dir)
		if err != nil {
			return nil, listOpenPRsOut{}, err
		}
		if err := client.CheckAuth(ctx); err != nil {
			return nil, listOpenPRsOut{}, err
		}
		prs, err := client.ListOpenPRs(ctx, repo, cfg.PRLimit)
		if err != nil {
			return nil, listOpenPRsOut{}, err
		}

		out := listOpenPRsOut{Repo: repo.Slug(), PRs: make([]prInfo, 0, len(prs))}
		for _, pr := range prs {
			out.PRs = append(out.PRs, toPRInfo(pr))
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_repo_review_tally",
		Description: "Get the CodeRabbit review-comment tally (total/resolved/open) per open PR and in aggregate, for the GitHub repo at the given directory (or the server's cwd if omitted).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in repoReviewTallyIn) (*mcp.CallToolResult, repoReviewTallyOut, error) {
		ctx, cancel := context.WithTimeout(ctx, cmdTimeout)
		defer cancel()

		repo, err := resolveRepo(ctx, in.Dir)
		if err != nil {
			return nil, repoReviewTallyOut{}, err
		}
		if err := client.CheckAuth(ctx); err != nil {
			return nil, repoReviewTallyOut{}, err
		}
		prs, err := client.ListOpenPRs(ctx, repo, cfg.PRLimit)
		if err != nil {
			return nil, repoReviewTallyOut{}, err
		}

		var agg tally.Tally
		out := repoReviewTallyOut{Repo: repo.Slug(), PRs: make([]prReviewTallyOut, 0, len(prs))}
		for _, pr := range prs {
			result, err := client.ReviewThreads(ctx, repo, pr.Number, cfg.BotLogin)
			if err != nil {
				return nil, repoReviewTallyOut{}, fmt.Errorf("PR #%d: %w", pr.Number, err)
			}
			t := tally.Count(result.Threads)
			agg = agg.Add(t)
			out.PRs = append(out.PRs, prReviewTallyOut{
				Number:    pr.Number,
				Title:     pr.Title,
				Tally:     toTallyInfo(t),
				Truncated: result.Truncated,
				Ambiguous: result.Ambiguous,
			})
		}
		out.Aggregate = toTallyInfo(agg)
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_pr_review_comments",
		Description: "Get every CodeRabbit review comment thread (resolved or open, with file path, body, and URL) on one pull request, for the GitHub repo at the given directory (or the server's cwd if omitted).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in prReviewCommentsIn) (*mcp.CallToolResult, prReviewCommentsOut, error) {
		ctx, cancel := context.WithTimeout(ctx, cmdTimeout)
		defer cancel()

		repo, err := resolveRepo(ctx, in.Dir)
		if err != nil {
			return nil, prReviewCommentsOut{}, err
		}
		if err := client.CheckAuth(ctx); err != nil {
			return nil, prReviewCommentsOut{}, err
		}
		result, err := client.ReviewThreads(ctx, repo, in.PRNumber, cfg.BotLogin)
		if err != nil {
			return nil, prReviewCommentsOut{}, err
		}

		out := prReviewCommentsOut{
			Repo:      repo.Slug(),
			PRNumber:  in.PRNumber,
			Threads:   make([]threadInfo, 0, len(result.Threads)),
			Truncated: result.Truncated,
			Ambiguous: result.Ambiguous,
		}
		for _, t := range result.Threads {
			out.Threads = append(out.Threads, toThreadInfo(t))
		}
		return nil, out, nil
	})

	return server
}

// Run starts the MCP server on stdio and blocks until the connection closes
// or ctx is cancelled.
func Run(ctx context.Context, client *ghclient.Client, cfg config.Config) error {
	server := New(client, cfg)
	return server.Run(ctx, &mcp.StdioTransport{})
}
