package ghclient

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type RunError struct {
	Cmd      string
	Stderr   string
	ExitCode int
	Err      error
}

func (e *RunError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("%s: exit %d", e.Cmd, e.ExitCode)
	}
	return fmt.Sprintf("%s: exit %d: %s", e.Cmd, e.ExitCode, e.Stderr)
}

func (e *RunError) Unwrap() error {
	return e.Err
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if len(stderrStr) > 2000 {
			stderrStr = stderrStr[:2000]
		}

		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}

		cmdStr := name
		if len(args) > 0 {
			cmdStr = name + " " + strings.Join(args, " ")
		}

		return nil, &RunError{
			Cmd:      cmdStr,
			Stderr:   stderrStr,
			ExitCode: exitCode,
			Err:      err,
		}
	}

	return stdout.Bytes(), nil
}

type Client struct {
	Runner Runner
}

func New() *Client {
	return &Client{
		Runner: ExecRunner{},
	}
}
