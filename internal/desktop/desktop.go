// Package desktop talks to the user's desktop: opening files, folders and
// URLs, and showing notifications, on macOS, Linux and Windows.
package desktop

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ErrUnsupported is returned when this system has no way to show a
// notification (for example Linux without notify-send).
var ErrUnsupported = errors.New("desktop notifications are not available on this system")

// Open opens a file, folder or URL with the system's default handler.
func Open(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return start("open", target)
	case "windows":
		return start("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		return start("xdg-open", target)
	}
}

// FileManager names the default file browser in messages.
func FileManager() string {
	switch runtime.GOOS {
	case "darwin":
		return "Finder"
	case "windows":
		return "Explorer"
	default:
		return "the file manager"
	}
}

// Notify shows a desktop notification. The text is passed as arguments, never
// spliced into a script.
func Notify(title, message string) error {
	switch runtime.GOOS {
	case "darwin":
		return runQuiet("osascript",
			"-e", "on run argv",
			"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
			"-e", "end run",
			title, message)
	case "linux":
		if _, err := exec.LookPath("notify-send"); err != nil {
			return ErrUnsupported
		}
		return runQuiet("notify-send", "--app-name=xplan", title, message)
	default:
		return ErrUnsupported
	}
}

// NotifyAvailable reports whether Notify can work on this system.
func NotifyAvailable() bool {
	switch runtime.GOOS {
	case "darwin":
		return true
	case "linux":
		_, err := exec.LookPath("notify-send")
		return err == nil
	default:
		return false
	}
}

func start(name string, args ...string) error {
	if err := exec.Command(name, args...).Start(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func runQuiet(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}
