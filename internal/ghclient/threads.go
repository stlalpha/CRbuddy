package ghclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stlalpha/prpal/internal/ghrepo"
)

// Thread is one review thread matching the requested author filter (a
// specific bot/user login, or every reviewer when botLogin is "").
type Thread struct {
	IsResolved  bool
	IsOutdated  bool
	Path        string // file path the thread is anchored to ("" for file-level/unknown)
	AuthorLogin string // login of the thread's first comment author
	URL         string // URL of the first comment ("" when absent)
	Body        string // body of the first comment
}

// ReviewThreadsResult is the outcome of a ReviewThreads fetch.
type ReviewThreadsResult struct {
	Threads []Thread
	// Ambiguous counts review threads whose first comment was missing or had
	// no author login, so bot ownership couldn't be determined. These are
	// excluded from Threads; a non-zero count means the tally may be undercounting.
	Ambiguous int
	// Truncated is true when pagination hit the hard page cap before GitHub
	// reported it was out of pages, so Threads/Ambiguous may be incomplete.
	Truncated bool
}

const threadsQuery = `
query($owner: String!, $name: String!, $number: Int!, $endCursor: String) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      reviewThreads(first: 100, after: $endCursor) {
        pageInfo { hasNextPage endCursor }
        nodes {
          isResolved
          isOutdated
          path
          comments(first: 1) {
            nodes {
              author { login }
              url
              body
            }
          }
        }
      }
    }
  }
}`

type threadsCommentAuthorJSON struct {
	Login string `json:"login"`
}

type threadsCommentJSON struct {
	Author *threadsCommentAuthorJSON `json:"author"`
	URL    string                    `json:"url"`
	Body   string                    `json:"body"`
}

type threadsCommentsJSON struct {
	Nodes []threadsCommentJSON `json:"nodes"`
}

type threadsNodeJSON struct {
	IsResolved bool                `json:"isResolved"`
	IsOutdated bool                `json:"isOutdated"`
	Path       string              `json:"path"`
	Comments   threadsCommentsJSON `json:"comments"`
}

type threadsPageInfoJSON struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type threadsReviewThreadsJSON struct {
	PageInfo threadsPageInfoJSON `json:"pageInfo"`
	Nodes    []threadsNodeJSON   `json:"nodes"`
}

type threadsPullRequestJSON struct {
	ReviewThreads threadsReviewThreadsJSON `json:"reviewThreads"`
}

type threadsRepositoryJSON struct {
	PullRequest *threadsPullRequestJSON `json:"pullRequest"`
}

type threadsDataJSON struct {
	Repository *threadsRepositoryJSON `json:"repository"`
}

type threadsResponseJSON struct {
	Data threadsDataJSON `json:"data"`
}

const (
	// maxPages caps pagination at 20 pages (2000 threads at 100/page) so a
	// runaway PR can't loop forever.
	maxPages = 20
	// perPageTimeout bounds a single page's GraphQL call, derived from the
	// context passed in, so a many-page PR isn't bound by one flat budget
	// shared across the whole fetch.
	perPageTimeout = 15 * time.Second
)

// ReviewThreads returns review threads on the PR, filtered by each thread's FIRST
// comment author. When botLogin is non-empty, only threads whose first comment's
// author login, lowercased, exactly matches botLogin or botLogin+"[bot]" (also
// lowercased) are included. When botLogin is "", every reviewer's threads are
// included (no author filtering) — useful for repos not using a specific bot.
func (c *Client) ReviewThreads(ctx context.Context, repo ghrepo.Repo, number int, botLogin string) (ReviewThreadsResult, error) {
	botLoginLower := strings.ToLower(botLogin)
	botAppLoginLower := botLoginLower + "[bot]"
	matchAll := botLoginLower == ""

	result := ReviewThreadsResult{Threads: make([]Thread, 0)}

	endCursor := ""
	hasCursor := false
	for page := 0; page < maxPages; page++ {
		args := []string{
			"api", "graphql",
			"-f", "query=" + threadsQuery,
			"-f", "owner=" + repo.Owner,
			"-f", "name=" + repo.Name,
			"-F", "number=" + strconv.Itoa(number),
		}
		if hasCursor {
			args = append(args, "-f", "endCursor="+endCursor)
		}

		pageCtx, cancel := context.WithTimeout(ctx, perPageTimeout)
		out, err := c.Runner.Run(pageCtx, "gh", args...)
		cancel()
		if err != nil {
			return ReviewThreadsResult{}, fmt.Errorf("fetch review threads for %s#%d: %w", repo.Slug(), number, err)
		}

		var resp threadsResponseJSON
		if err := json.Unmarshal(out, &resp); err != nil {
			return ReviewThreadsResult{}, fmt.Errorf("parse graphql response for #%d: %w", number, err)
		}

		if resp.Data.Repository == nil || resp.Data.Repository.PullRequest == nil {
			return ReviewThreadsResult{}, fmt.Errorf("PR #%d not found in %s", number, repo.Slug())
		}

		rt := resp.Data.Repository.PullRequest.ReviewThreads
		for _, node := range rt.Nodes {
			if len(node.Comments.Nodes) == 0 {
				result.Ambiguous++
				continue
			}
			first := node.Comments.Nodes[0]
			if first.Author == nil || first.Author.Login == "" {
				result.Ambiguous++
				continue
			}
			login := strings.ToLower(first.Author.Login)
			if !matchAll && login != botLoginLower && login != botAppLoginLower {
				continue
			}
			result.Threads = append(result.Threads, Thread{
				IsResolved:  node.IsResolved,
				IsOutdated:  node.IsOutdated,
				Path:        node.Path,
				AuthorLogin: first.Author.Login,
				URL:         first.URL,
				Body:        first.Body,
			})
		}

		if !rt.PageInfo.HasNextPage {
			break
		}
		if page == maxPages-1 {
			result.Truncated = true
			break
		}
		endCursor = rt.PageInfo.EndCursor
		hasCursor = true
	}

	return result, nil
}
