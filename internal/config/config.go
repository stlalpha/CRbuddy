package config

import (
	"flag"
	"fmt"
	"time"
)

// Config holds runtime settings for prpal.
type Config struct {
	// RefreshInterval is how often the TUI auto-refreshes. Default 30s. Minimum 5s (Load returns an error below that).
	RefreshInterval time.Duration
	// BotLogin is matched (case-insensitive, exact) against a review thread's first-comment
	// author login, or that login with a "[bot]" suffix, to decide which reviewer's threads
	// are tracked. Default "coderabbitai". An empty string tracks every reviewer's threads
	// (no author filtering) — useful on repos not using a specific bot.
	BotLogin string
	// PRLimit caps how many open PRs are fetched. Default 50.
	PRLimit int
}

// Default returns the built-in defaults: RefreshInterval 30s, BotLogin "coderabbitai", PRLimit 50.
func Default() Config {
	return Config{
		RefreshInterval: 30 * time.Second,
		BotLogin:        "coderabbitai",
		PRLimit:         50,
	}
}

// Load parses args (os.Args[1:]) with a flag.NewFlagSet("prpal", flag.ContinueOnError):
//
//	-refresh duration  (default 30s)  e.g. -refresh 1m
//	-bot string        (default "coderabbitai")
//	-limit int         (default 50)
//
// Returns Default() overlaid with parsed flags. Errors: flag parse errors returned as-is
// (flag.ErrHelp included — main treats any error as exit 2); refresh < 5s returns
// fmt.Errorf("refresh interval must be at least 5s").
func Load(args []string) (Config, error) {
	cfg := Default()

	fs := flag.NewFlagSet("prpal", flag.ContinueOnError)
	fs.DurationVar(&cfg.RefreshInterval, "refresh", cfg.RefreshInterval, "auto-refresh interval")
	fs.StringVar(&cfg.BotLogin, "bot", cfg.BotLogin, "reviewer login to match (exact, or with a [bot] suffix); \"\" tracks every reviewer")
	fs.IntVar(&cfg.PRLimit, "limit", cfg.PRLimit, "max open PRs to fetch")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if cfg.RefreshInterval < 5*time.Second {
		return Config{}, fmt.Errorf("refresh interval must be at least 5s")
	}

	return cfg, nil
}
