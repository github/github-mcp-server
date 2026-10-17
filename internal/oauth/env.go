package oauth

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// errNoDisplay reports that the host has no display server, so no browser can be
// launched. It is a definitive headless signal (unlike a generic launch error),
// which lets the flow prefer device authorization — the only channel reachable
// from a browser on another machine (e.g. a remote SSH session).
var errNoDisplay = errors.New("no display server detected")

// openBrowser tries to open url in the user's default browser. It returns an
// error when no browser can plausibly be launched so the caller can fall back
// to elicitation. On Linux it treats a headless session (no display server) as
// unopenable, which is the common case for SSH and containers.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return errNoDisplay
		}
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	// The launcher (xdg-open/open/rundll32) exits as soon as it hands off to the
	// browser. Reap it asynchronously so it does not linger as a zombie for the
	// lifetime of this long-running server.
	go func() { _ = cmd.Wait() }()
	return nil
}

// isRunningInDocker reports whether the process is running inside a Docker (or
// containerd) container. Detection relies on Linux-specific paths and is always
// false elsewhere. It is used only to skip a PKCE flow that cannot work: a
// random callback port inside a container cannot be reached from the host
// browser, so we go straight to device flow in that case.
func isRunningInDocker() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(data)
		if strings.Contains(s, "docker") || strings.Contains(s, "containerd") {
			return true
		}
	}
	return false
}
