package ghclient

import "context"

// fakeRunner is a Runner whose behavior is entirely determined by RunFunc,
// for testing ghclient methods without shelling out to a real gh/git.
type fakeRunner struct {
	RunFunc func(ctx context.Context, name string, args ...string) ([]byte, error)
	calls   [][]string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.RunFunc(ctx, name, args...)
}
