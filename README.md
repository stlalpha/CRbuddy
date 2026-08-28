# CodeRabbit Buddy

[![CI](https://github.com/stlalpha/CRbuddy/actions/workflows/ci.yml/badge.svg)](https://github.com/stlalpha/CRbuddy/actions/workflows/ci.yml)

A terminal companion that runs in a separate window while you work in a GitHub repo. It shows your open pull requests and a running tally of CodeRabbit's review comments (resolved vs. open), auto-refreshing in the background. It also exposes the same data as MCP tools, so an agent can query it directly.

## Requirements

- Go 1.25.1+ (see `go.mod`)
- [`gh`](https://cli.github.com) installed and authenticated (`gh auth login`)
- A git repo with a GitHub remote

## Install

```bash
go install github.com/stlalpha/CRbuddy@latest
```

Or build from a clone:

```bash
git clone https://github.com/stlalpha/CRbuddy.git
cd CRbuddy
go build -o coderabbit-buddy .
```

## TUI

Run it from inside the repo you want to watch:

```bash
coderabbit-buddy
```

On startup it detects the repo from your git remote (`origin`, falling back to any other remote that parses as a GitHub URL), checks that `gh` is installed and authenticated, then lists open PRs and fetches each one's CodeRabbit review threads via `gh api graphql`. A thread counts as CodeRabbit's only if its first comment's author login exactly matches the bot login, or that login with a `[bot]` suffix (not a substring match). A thread counts as resolved only when GitHub reports its review thread as resolved.

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `-refresh` | `30s` | Auto-refresh interval (minimum `5s`) |
| `-bot` | `coderabbitai` | Bot login to match against comment authors |
| `-limit` | `50` | Max open PRs to fetch |

### Keys

| Key | Action |
|---|---|
| `↑`/`k`, `↓`/`j` | Move the selection |
| `enter` | Open the selected PR's comment-thread table |
| `esc` or `enter` (in the table) | Back to the PR list |
| `o` (in the table) | Open the selected thread's URL in the browser (macOS, Linux, and Windows; errors show inline instead of failing silently) |
| `r` | Manual refresh |
| `q` / `ctrl+c` | Quit |

The PR list shows number, title, author, last-updated time, and a per-PR tally (`✔ resolved` / `● open` / total), with `⚠ truncated` or `⚠ N ambiguous` warnings when a fetch may be incomplete. The header shows the repo, an aggregate tally across all open PRs, and `(partial)` if any row hasn't finished loading, errored, or came back truncated/ambiguous. The comment table lists every CodeRabbit thread on the selected PR, resolved and open, with file path and a body preview.

## MCP server

```bash
coderabbit-buddy mcp
```

Starts an MCP server over stdio instead of the TUI. It's read-only and reuses the same repo-detection, PR-fetching, and tally logic as the TUI. `-bot` and `-limit` apply here too; `-refresh` does not (tools are called on demand, not polled).

Register it with Claude Code:

```bash
claude mcp add coderabbit-buddy -- /path/to/coderabbit-buddy mcp
```

### Tools

| Tool | Input | Output |
|---|---|---|
| `list_open_prs` | `dir` (optional, defaults to cwd) | `{ repo, prs: [{ number, title, author, url, head_ref, is_draft, updated_at }] }` |
| `get_repo_review_tally` | `dir` (optional) | `{ repo, aggregate: { total, resolved, open }, prs: [{ number, title, tally, truncated?, ambiguous? }] }` |
| `get_pr_review_comments` | `dir` (optional), `pr_number` | `{ repo, pr_number, threads: [{ is_resolved, is_outdated, path, body, url }], truncated?, ambiguous? }` |

`dir` resolves the repo the same way the TUI does (git remote detection from that directory); it does not accept an `owner/repo` string directly. Errors (not a git repo, `gh` not authenticated, PR not found) come back as normal MCP tool errors, not crashes.

## Troubleshooting

**"gh is not authenticated" but you know you ran `gh auth login`.** Run `gh auth status` to confirm which account and scopes are active. If your org requires SSO, the token needs to be authorized for it: `gh auth refresh` re-runs the login flow, or authorize the existing token from https://github.com/settings/tokens against the org.

**PRs load fine but every PR shows 0 CodeRabbit comments when you know it has some.** The GraphQL query needs the same access a normal `repo` scope grants. Check with `gh auth status`; if the scope list is missing `repo` (e.g. you only have `read:org`), re-auth with `gh auth refresh -s repo`.

**"gh: Could not resolve to a PullRequest with the number of N".** Either the PR number doesn't exist in the detected repo, or you don't have access to it (private repo, no permission). The tool prints the repo slug it resolved (in the TUI header, or the `repo` field in an MCP tool's output) — confirm that's the repo you meant.

**A PR shows `⚠ truncated` or `⚠ N ambiguous`.** Truncated means the PR has more than 2000 review comment threads (the pagination cap); the tally is a lower bound. Ambiguous means some threads had no readable first-comment author and couldn't be attributed to CodeRabbit or anyone else; they're excluded from the tally rather than guessed at.

**Nothing happens on a large repo, or it's slow.** Each open PR is a separate `gh api graphql` call; a repo with many open PRs means many sequential calls per refresh. Lower `-limit` to fetch fewer PRs, or raise `-refresh` to poll less often.

**`o` says "opening a browser is not supported" or the command fails.** It shells out to `open` (macOS), `xdg-open` (Linux), or `rundll32` (Windows). On a minimal Linux install without a desktop environment, `xdg-open` may not exist; install it or open the printed URL manually.

## Layout

```
main.go                    entrypoint: dispatches to the TUI or `mcp` subcommand
internal/config/           flags and defaults
internal/ghrepo/           git remote → GitHub owner/repo detection
internal/ghclient/         gh CLI wrapper: auth check, PR list, review-thread GraphQL fetch
internal/tally/            resolved/open/total counting
internal/tui/              Bubble Tea model, view, and key handling
internal/mcpserver/        MCP tool definitions, wrapping ghclient/tally
```

## Status

`go build`, `go vet`, `gofmt`, and `go test ./...` run in CI on every push/PR to `main`. Unit tests cover the GitHub-remote parsing, bot matching, pagination/truncation, tally math, and TUI keybinding logic (`internal/config`, `internal/ghclient`, `internal/ghrepo`, `internal/tally`, `internal/tui` all have real coverage); the MCP tool wiring itself is thinner on unit tests but has been exercised live against a real repo.

## License

MIT, see [LICENSE](LICENSE).
