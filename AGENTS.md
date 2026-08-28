# Agent instructions

## Commands

```bash
go build ./...          # build
go vet ./...             # vet
gofmt -l .                # list unformatted files (should print nothing); gofmt -w to fix
go test ./...             # unit tests
go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...   # vuln scan
```

All five run in CI (`.github/workflows/ci.yml`) on every push/PR to `main`. Run them locally before pushing — CI is not a substitute for checking your own work.

## Layout

```
main.go                    entrypoint: dispatches to the TUI or the `mcp` subcommand
internal/config/           flags and defaults
internal/ghrepo/           git remote -> GitHub owner/repo detection
internal/ghclient/         gh CLI wrapper: auth check, PR list, review-thread GraphQL fetch
internal/tally/            resolved/open/total counting
internal/tui/              Bubble Tea model, view, and key handling
internal/mcpserver/        MCP tool definitions, wrapping ghclient/tally
```

## Conventions

- **All GitHub access goes through the `gh` CLI**, via `ghclient.Client.Runner` (an interface). Never call the GitHub REST/GraphQL API directly with an HTTP client — the whole point is reusing the user's existing `gh auth` session instead of managing tokens. If you need a new GitHub interaction, add a method on `Client` that shells out through `Runner`.
- **Test `gh`-shelling code with a fake `Runner`**, not real `gh` calls. See `internal/ghclient/fake_runner_test.go` for the pattern (`RunFunc func(...) ([]byte, error)`), and `prs_test.go`/`threads_test.go` for how it's used to test JSON parsing, pagination, and error paths without touching the network. `internal/ghrepo`'s tests are the exception — they exercise real local `git` in a temp dir (no network, no GitHub), which is fine.
- **Sentinel errors for anything a caller needs to branch on.** See `ghrepo.ErrNotGitRepo`, `ghclient.ErrGHNotAuthenticated`, etc. — package-level `var Err... = errors.New(...)`, wrapped with `fmt.Errorf("...: %w", err)` at each layer, matched with `errors.Is`/`errors.As`. Don't match on error message strings.
- **The Bubble Tea `Model` uses value receivers throughout**, per Bubble Tea's own convention — `Update` returns a new `Model` value, it doesn't mutate through a pointer. Keep new state on the struct, not in package-level vars.
- **Context lifetime is explicit and layered**: `Model.ctx` lives for the program's lifetime (cancelled on quit), `Model.genCtx` is per-refresh-generation (cancelled when a new refresh starts, killing the previous generation's in-flight `gh` subprocesses), and each `gh` call gets its own short timeout derived from whichever context it received (`cmdTimeout` in `messages.go`, `perPageTimeout` in `threads.go`). When adding a new `gh`-shelling call from the TUI, thread `genCtx` through rather than reaching for `context.Background()`.
- **Reviewer matching is exact, not substring**: a comment author only matches `-bot` if its login equals it (case-insensitively) or that login plus `[bot]`. This was a real bug once (`strings.Contains`); don't reintroduce it. The one deliberate escape hatch is `botLogin == ""`, which means "every reviewer, no filtering" (see `matchAll` in `threads.go`) — that's intentional, not a bug to "fix" by requiring a non-empty value.
- **Pagination truncation and ambiguous-author counts are signals, not swallowed.** `ReviewThreads` returns `Truncated`/`Ambiguous` on its result; every consumer (TUI rows, MCP tool output) surfaces them rather than silently under-reporting the tally.
- **The MCP server (`internal/mcpserver`) must stay a thin wrapper.** It should never duplicate GitHub-fetching or tally logic — every tool handler resolves the repo, then calls into `ghclient`/`tally` the same way the TUI does. If a tool needs new data, add it to `ghclient`/`tally` first, then wrap it.
- **Comments explain the non-obvious, not the obvious.** Match the existing style: doc comments on exported types/functions describing behavior and invariants (e.g. exact `gh` commands used, error-wrapping conventions), not narration of what a line of code does.

## Gotchas

- No `LICENSE`-adjacent secrets or tokens live in this repo — auth is entirely delegated to the user's own `gh auth login` session. Never add a hardcoded token, and never suggest committing one for "convenience."
- The module is `github.com/stlalpha/prpal`; `go.mod` pins a `toolchain` line (currently `go1.25.13`) specifically to stay ahead of known stdlib CVEs — don't remove it without checking `govulncheck ./...` still passes on whatever toolchain replaces it.
- There's no `CLAUDE.md` in this repo on purpose (it's gitignored) — personal assistant instructions aren't checked in here. This file is the one meant for any agent working in this codebase, Claude or otherwise.
