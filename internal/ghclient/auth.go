package ghclient

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

var (
	ErrGHNotInstalled     = errors.New("gh CLI is not installed (https://cli.github.com)")
	ErrGHNotAuthenticated = errors.New("gh is not authenticated — run `gh auth status` / `gh auth login`")
)

func (c *Client) CheckAuth(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return ErrGHNotInstalled
	}

	_, err := c.Runner.Run(ctx, "gh", "auth", "status")
	if err != nil {
		var runErr *RunError
		if errors.As(err, &runErr) {
			return fmt.Errorf("%w: %s", ErrGHNotAuthenticated, runErr.Stderr)
		}
		return fmt.Errorf("%w: %v", ErrGHNotAuthenticated, err)
	}

	return nil
}
