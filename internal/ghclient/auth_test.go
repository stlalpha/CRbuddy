package ghclient

import (
	"context"
	"errors"
	"testing"
)

func TestCheckAuth_Success(t *testing.T) {
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("logged in"), nil
		},
	}}
	if err := c.CheckAuth(context.Background()); err != nil {
		t.Fatalf("CheckAuth() error = %v, want nil", err)
	}
}

func TestCheckAuth_NotAuthenticated(t *testing.T) {
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, &RunError{Cmd: "gh auth status", Stderr: "not logged in", ExitCode: 1, Err: errors.New("exit status 1")}
		},
	}}
	err := c.CheckAuth(context.Background())
	if !errors.Is(err, ErrGHNotAuthenticated) {
		t.Fatalf("CheckAuth() error = %v, want ErrGHNotAuthenticated", err)
	}
}

func TestCheckAuth_GHNotInstalled(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	c := &Client{Runner: &fakeRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			t.Fatal("Runner.Run should not be called when gh isn't on PATH")
			return nil, nil
		},
	}}
	err := c.CheckAuth(context.Background())
	if !errors.Is(err, ErrGHNotInstalled) {
		t.Fatalf("CheckAuth() error = %v, want ErrGHNotInstalled", err)
	}
}
