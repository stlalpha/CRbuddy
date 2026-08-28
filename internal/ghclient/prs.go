package ghclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/stlalpha/CRbuddy/internal/ghrepo"
)

// PR is one open pull request.
type PR struct {
	Number    int
	Title     string
	Author    string // login only
	URL       string
	HeadRef   string
	IsDraft   bool
	UpdatedAt time.Time
}

// prJSON mirrors the shape gh emits for --json number,title,author,url,headRefName,isDraft,updatedAt
type prJSON struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
	URL         string    `json:"url"`
	HeadRefName string    `json:"headRefName"`
	IsDraft     bool      `json:"isDraft"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ListOpenPRs returns open PRs in gh's default order (by creation, not by UpdatedAt).
func (c *Client) ListOpenPRs(ctx context.Context, repo ghrepo.Repo, limit int) ([]PR, error) {
	out, err := c.Runner.Run(ctx, "gh", "pr", "list",
		"--repo", repo.Slug(),
		"--state", "open",
		"--limit", strconv.Itoa(limit),
		"--json", "number,title,author,url,headRefName,isDraft,updatedAt",
	)
	if err != nil {
		return nil, fmt.Errorf("list PRs for %s: %w", repo.Slug(), err)
	}

	var raw []prJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse gh pr list output: %w", err)
	}

	prs := make([]PR, 0, len(raw))
	for _, r := range raw {
		author := ""
		if r.Author != nil {
			author = r.Author.Login
		}
		prs = append(prs, PR{
			Number:    r.Number,
			Title:     r.Title,
			Author:    author,
			URL:       r.URL,
			HeadRef:   r.HeadRefName,
			IsDraft:   r.IsDraft,
			UpdatedAt: r.UpdatedAt,
		})
	}

	return prs, nil
}
