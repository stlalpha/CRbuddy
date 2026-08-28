package ghclient

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunError_Error(t *testing.T) {
	cases := []struct {
		name string
		err  *RunError
		want string
	}{
		{"with stderr", &RunError{Cmd: "gh pr list", Stderr: "boom", ExitCode: 1}, "gh pr list: exit 1: boom"},
		{"no stderr", &RunError{Cmd: "gh pr list", Stderr: "", ExitCode: 2}, "gh pr list: exit 2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.err.Error(); got != c.want {
				t.Errorf("Error() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRunError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	err := &RunError{Err: inner}
	if !errors.Is(err, inner) {
		t.Error("errors.Is(err, inner) = false, want true")
	}
}

func TestExecRunner_Success(t *testing.T) {
	out, err := (ExecRunner{}).Run(context.Background(), "go", "version")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.HasPrefix(string(out), "go version") {
		t.Errorf("Run() output = %q, want prefix %q", out, "go version")
	}
}

func TestExecRunner_NonZeroExit(t *testing.T) {
	_, err := (ExecRunner{}).Run(context.Background(), "go", "definitely-not-a-real-subcommand")
	if err == nil {
		t.Fatal("Run() with a bad subcommand: want an error, got nil")
	}
	var runErr *RunError
	if !errors.As(err, &runErr) {
		t.Fatalf("Run() error = %v, want a *RunError", err)
	}
	if runErr.ExitCode == 0 {
		t.Error("ExitCode = 0, want non-zero")
	}
	if runErr.Stderr == "" {
		t.Error("Stderr is empty, want the command's error output")
	}
}
