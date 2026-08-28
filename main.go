package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stlalpha/CRbuddy/internal/config"
	"github.com/stlalpha/CRbuddy/internal/ghclient"
	"github.com/stlalpha/CRbuddy/internal/mcpserver"
	"github.com/stlalpha/CRbuddy/internal/tui"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "mcp" {
		runMCP(args[1:])
		return
	}
	runTUI(args)
}

// runMCP starts the MCP server on stdio; it blocks until the connection closes.
func runMCP(args []string) {
	cfg, err := config.Load(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	client := ghclient.New()
	if err := mcpserver.Run(context.Background(), client, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runTUI(args []string) {
	cfg, err := config.Load(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	client := ghclient.New()
	m := tui.New(cfg, client, cwd)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
