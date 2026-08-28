package tui

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openURL opens url in the user's default browser. Behavior varies by OS:
// macOS uses `open`, Linux uses `xdg-open`, Windows uses `rundll32` via its
// URL protocol handler. Returns an error (never panics) if the platform is
// unsupported or the command fails to start.
func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("opening a browser is not supported on %s", runtime.GOOS)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
