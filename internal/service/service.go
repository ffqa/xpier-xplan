// Package service installs xplan as a per-user background service that starts
// at login and restarts after a crash: launchd on macOS, systemd --user on
// Linux. Scheduled reviews only run while `xplan serve` is running, so this is
// what makes them happen without a terminal left open.
package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ffqa/xpier-xplan/internal/config"
)

// ErrUnsupported is returned on systems without a supported service manager.
var ErrUnsupported = errors.New("background service is only supported on macOS (launchd) and Linux (systemd --user)")

// Spec describes the service to install.
type Spec struct {
	Binary  string // absolute path of the xplan executable
	Home    string // xplan home passed as --home
	Listen  string // address the server listens on, used for health checks
	LogFile string // file the server writes its log to
	Path    string // PATH for agents started by reviews
}

// Status is what `xplan service status` reports.
type Status struct {
	Manager   string // launchd or systemd
	Installed bool   // the unit file exists
	Running   bool   // the service manager reports a live process
	PID       int
	UnitFile  string
}

// Manager controls the service through the platform's service manager.
type Manager interface {
	Name() string
	UnitFile() string
	Install(spec Spec) error
	Uninstall() error
	Restart() error
	Status() (Status, error)
}

// ForCurrentOS returns the manager for this platform.
func ForCurrentOS() (Manager, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	switch runtime.GOOS {
	case "darwin":
		return &launchd{userHome: userHome, uid: os.Getuid()}, nil
	case "linux":
		return &systemd{userHome: userHome}, nil
	default:
		return nil, ErrUnsupported
	}
}

// NewSpec fills a Spec from the running executable.
func NewSpec(home, listen, path string) (Spec, error) {
	executable, err := os.Executable()
	if err != nil {
		return Spec{}, fmt.Errorf("find the xplan executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	if strings.HasPrefix(executable, os.TempDir()) || strings.Contains(executable, "/go-build") {
		return Spec{}, fmt.Errorf("%s is a temporary build; build or install xplan first (make build), then run this command from that binary", executable)
	}
	return Spec{
		Binary:  executable,
		Home:    home,
		Listen:  listen,
		LogFile: filepath.Join(home, "logs", "serve.log"),
		Path:    path,
	}, nil
}

// PortInUse reports whether something already accepts connections on listen.
func PortInUse(listen string) bool {
	conn, err := net.DialTimeout("tcp", config.DialAddress(listen), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// WaitForPortFree waits until nothing listens on listen, up to timeout.
func WaitForPortFree(listen string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !PortInUse(listen) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return !PortInUse(listen)
}

// WaitForServer waits until the xplan API answers on listen, up to timeout.
func WaitForServer(listen string, timeout time.Duration) error {
	url := config.LocalURL(listen) + "/api/meta"
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		response, err := http.DefaultClient.Do(request)
		cancel()
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("status %d", response.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("xplan did not answer on %s within %s: %v", url, timeout, lastErr)
}

// run executes a service manager command and folds its output into the error.
func run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text != "" {
			return text, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, text)
		}
		return text, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return text, nil
}

func writeUnit(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func ensureLogDir(spec Spec) error {
	if err := os.MkdirAll(filepath.Dir(spec.LogFile), 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
