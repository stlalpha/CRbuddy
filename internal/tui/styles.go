package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/stlalpha/prpal/internal/ghclient"
	"github.com/stlalpha/prpal/internal/ghrepo"
	"github.com/stlalpha/prpal/internal/tally"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true)
	dimStyle       = lipgloss.NewStyle().Faint(true)
	colHeaderStyle = lipgloss.NewStyle().Bold(true).Faint(true)
	openStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	resolvedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	errBox         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("9")).Padding(1)
	draftStyle     = lipgloss.NewStyle().Faint(true)
	cursorStyle    = lipgloss.NewStyle().Bold(true)
)

// Column layout for the PR table. cursorColWidth/numColWidth/authorColWidth/
// updatedColWidth are fixed; the title column takes whatever room is left
// after those plus a reserve for the trailing status text (tally counts,
// draft marker, errors).
const (
	cursorColWidth  = 2
	numColWidth     = 6
	authorColWidth  = 12
	updatedColWidth = 10
	statusReserve   = 26
	colGap          = 2
)

// titleColWidth returns how wide the title column should be for a given
// terminal width, leaving room for the other fixed-width columns and the
// status text that trails each row.
func titleColWidth(width int) int {
	reserved := cursorColWidth + numColWidth + authorColWidth + updatedColWidth + statusReserve + colGap*4
	w := width - reserved
	if w < 10 {
		w = 10
	}
	return w
}

// padCol truncates s to w runes (with ellipsis if needed) and pads it with
// trailing spaces so fixed-width columns line up.
func padCol(s string, w int) string {
	return lipgloss.NewStyle().Width(w).Render(truncate(s, w))
}

// formatUpdated renders a timestamp as a terse, relative-ish string for a
// narrow TUI column: "45m ago", "6h ago", "3d ago", falling back to a short
// date once it's more than a week old.
func formatUpdated(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	elapsed := time.Since(t)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	case elapsed < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// reviewerLabel returns the display name for the configured reviewer filter:
// "CodeRabbit" for the default bot login, "Reviews" when tracking every
// reviewer (botLogin == ""), or the literal login otherwise.
func reviewerLabel(botLogin string) string {
	switch {
	case botLogin == "":
		return "Reviews"
	case strings.EqualFold(botLogin, "coderabbitai"):
		return "CodeRabbit"
	default:
		return botLogin
	}
}

func renderRow(r prRow, selected bool, label string, width int) string {
	prNum := fmt.Sprintf("#%d", r.pr.Number)
	titleWidth := titleColWidth(width)

	cursorMark := "  "
	if selected {
		cursorMark = cursorStyle.Render("> ")
	}
	numCol := padCol(prNum, numColWidth)
	titleCol := padCol(r.pr.Title, titleWidth)
	authorCol := padCol(r.pr.Author, authorColWidth)
	updatedCol := padCol(formatUpdated(r.pr.UpdatedAt), updatedColWidth)

	base := cursorMark + strings.Repeat(" ", colGap) + strings.Join([]string{numCol, titleCol, authorCol, updatedCol}, strings.Repeat(" ", colGap))

	if !r.loaded && r.err == nil {
		return base + "  " + dimStyle.Render("…")
	}

	if r.err != nil {
		errMsg := r.err.Error()
		if idx := strings.IndexByte(errMsg, '\n'); idx >= 0 {
			errMsg = errMsg[:idx]
		}
		return base + "  " + errStyle.Render("error: "+errMsg)
	}

	if !r.loaded {
		return base
	}

	warn := ""
	if r.truncated {
		warn += "  " + errStyle.Render("⚠ truncated")
	}
	if r.ambiguous > 0 {
		warn += "  " + errStyle.Render(fmt.Sprintf("⚠ %d ambiguous", r.ambiguous))
	}

	if r.tally.Total == 0 {
		return base + "  " + dimStyle.Render(fmt.Sprintf("%s: none", label)) + warn
	}

	if r.tally.Done() {
		return base + "  " + resolvedStyle.Render(fmt.Sprintf("✔ all %d resolved", r.tally.Total)) + warn
	}

	if r.pr.IsDraft {
		base += "  " + draftStyle.Render("[draft]")
	}

	base += "  " + resolvedStyle.Render(fmt.Sprintf("✔ %d", r.tally.Resolved))
	base += "  " + openStyle.Render(fmt.Sprintf("● %d", r.tally.Open))
	base += "  " + fmt.Sprintf("(%d)", r.tally.Total)
	base += warn

	return base
}

func renderHeader(repo ghrepo.Repo, agg tally.Tally, partial bool, lastOK time.Time, refreshing bool, spin string, label string, width int) string {
	line1 := titleStyle.Render(repo.Slug())
	if refreshing {
		line1 += " " + spin
	}

	var aggStr string
	if agg.Total == 0 {
		aggStr = label + ": no threads"
	} else {
		aggStr = fmt.Sprintf("%s: %s open / %s resolved / %d total", label,
			openStyle.Render(fmt.Sprintf("%d", agg.Open)),
			resolvedStyle.Render(fmt.Sprintf("%d", agg.Resolved)),
			agg.Total)
	}
	if partial {
		aggStr += " " + errStyle.Render("(partial)")
	}

	var timeStr string
	if lastOK.IsZero() {
		timeStr = "never refreshed"
	} else {
		elapsed := time.Since(lastOK)
		if elapsed < time.Minute {
			timeStr = fmt.Sprintf("%ds ago", int(elapsed.Seconds()))
		} else if elapsed < time.Hour {
			timeStr = fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
		} else {
			timeStr = fmt.Sprintf("%dh ago", int(elapsed.Hours()))
		}
	}

	line2 := aggStr + "  " + dimStyle.Render(timeStr)

	titleWidth := titleColWidth(width)
	cols := padCol("", cursorColWidth) + strings.Repeat(" ", colGap) + strings.Join([]string{
		padCol("PR", numColWidth),
		padCol("TITLE", titleWidth),
		padCol("AUTHOR", authorColWidth),
		padCol("UPDATED", updatedColWidth),
	}, strings.Repeat(" ", colGap))
	line3 := colHeaderStyle.Render(cols)

	return line1 + "\n" + line2 + "\n" + line3
}

// Column layout for the thread detail table.
const (
	threadCursorColWidth = 2
	statusColWidth       = 10
	pathColWidth         = 28
)

// threadCommentColWidth returns how wide the comment-preview column should be
// for a given terminal width, leaving room for the other fixed-width columns.
func threadCommentColWidth(width int) int {
	reserved := threadCursorColWidth + statusColWidth + authorColWidth + pathColWidth + colGap*4
	w := width - reserved
	if w < 15 {
		w = 15
	}
	return w
}

// renderThreadTable renders the detail view for one PR: every matching review
// thread on it (both resolved and open, from whichever reviewer(s) are
// configured), with a clear status indicator, author, file path, and a
// truncated comment preview. cursor selects a row for the "o" (open in
// browser) keybinding.
func renderThreadTable(pr ghclient.PR, threads []ghclient.Thread, cursor int, label string, width int) string {
	title := titleStyle.Render(fmt.Sprintf("#%d — %s", pr.Number, pr.Title))

	if len(threads) == 0 {
		return title + "\n\n" + dimStyle.Render(fmt.Sprintf("no %s threads on this PR", label))
	}

	commentWidth := threadCommentColWidth(width)
	header := padCol("", threadCursorColWidth) + strings.Repeat(" ", colGap) +
		strings.Join([]string{
			padCol("STATUS", statusColWidth),
			padCol("AUTHOR", authorColWidth),
			padCol("PATH", pathColWidth),
			padCol("COMMENT", commentWidth),
		}, strings.Repeat(" ", colGap))

	lines := []string{title, "", colHeaderStyle.Render(header)}
	for i, t := range threads {
		mark := "  "
		if i == cursor {
			mark = cursorStyle.Render("> ")
		}

		var statusCol string
		if t.IsResolved {
			statusCol = resolvedStyle.Render(padCol("✔ resolved", statusColWidth))
		} else {
			statusCol = openStyle.Render(padCol("● open", statusColWidth))
		}

		author := t.AuthorLogin
		if author == "" {
			author = "?"
		}
		authorCol := padCol(author, authorColWidth)

		path := t.Path
		if path == "" {
			path = "(file-level)"
		}
		pathCol := padCol(path, pathColWidth)
		commentCol := padCol(strings.ReplaceAll(t.Body, "\n", " "), commentWidth)

		row := mark + strings.Repeat(" ", colGap) + strings.Join([]string{statusCol, authorCol, pathCol, commentCol}, strings.Repeat(" ", colGap))
		lines = append(lines, row)
	}

	return strings.Join(lines, "\n")
}

func truncate(s string, max int) string {
	if max < 1 {
		return "…"
	}

	runes := []rune(s)
	if len(runes) <= max {
		return s
	}

	if max == 1 {
		return "…"
	}

	return string(runes[:max-1]) + "…"
}
