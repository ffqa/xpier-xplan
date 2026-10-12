package review

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ffqa/xpier-xplan/internal/config"
)

// runResult is what one agent invocation produced.
type runResult struct {
	Output   string
	ExitCode int
	Err      error
}

// extraPathDirs are appended to PATH so agents installed in user locations are
// found even when xplan is started from launchd with a minimal environment.
func extraPathDirs() []string {
	userHome, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(userHome, ".local", "bin"),
		"/opt/homebrew/bin",
		"/usr/local/bin",
		filepath.Join(userHome, ".opencode", "bin"),
	}
	nvmBins, _ := filepath.Glob(filepath.Join(userHome, "Library", "Application Support", "Herd", "config", "nvm", "versions", "node", "*", "bin"))
	return append(dirs, nvmBins...)
}

// AugmentedPath returns PATH with the extra agent directories appended.
func AugmentedPath() string {
	parts := []string{os.Getenv("PATH")}
	parts = append(parts, extraPathDirs()...)
	return strings.Join(parts, string(os.PathListSeparator))
}

// LookupAgent resolves an agent binary against the augmented PATH.
func LookupAgent(binary string) (string, error) {
	if strings.Contains(binary, "/") {
		if _, err := os.Stat(binary); err != nil {
			return "", fmt.Errorf("agent binary %s not found", binary)
		}
		return binary, nil
	}
	for _, dir := range filepath.SplitList(AugmentedPath()) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, binary)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("agent %q is not installed (not on PATH)", binary)
}

// BuildArgv substitutes {prompt}, {prompt_file} and {path} in a reviewer template.
func BuildArgv(def config.ReviewerDef, prompt, promptFile, projectPath string) ([]string, error) {
	if len(def.Command) == 0 {
		return nil, errors.New("reviewer has an empty command")
	}
	argv := make([]string, 0, len(def.Command))
	for _, part := range def.Command {
		replaced := strings.ReplaceAll(part, "{path}", projectPath)
		replaced = strings.ReplaceAll(replaced, "{prompt_file}", promptFile)
		replaced = strings.ReplaceAll(replaced, "{prompt}", prompt)
		argv = append(argv, replaced)
	}
	return argv, nil
}

// runAgent executes argv in projectPath, streaming stdout into outputPath and
// stderr into logPath. The whole process group is killed on cancel/timeout.
func runAgent(ctx context.Context, argv []string, projectPath, outputPath, logPath string, timeout time.Duration) runResult {
	binary, err := LookupAgent(argv[0])
	if err != nil {
		return runResult{ExitCode: -1, Err: err}
	}
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return runResult{ExitCode: -1, Err: fmt.Errorf("create output file: %w", err)}
	}
	defer outputFile.Close()
	logFile, err := os.Create(logPath)
	if err != nil {
		return runResult{ExitCode: -1, Err: fmt.Errorf("create log file: %w", err)}
	}
	defer logFile.Close()

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, binary, argv[1:]...)
	cmd.Dir = projectPath
	cmd.Env = append(os.Environ(), "PATH="+AugmentedPath(), "NO_COLOR=1", "CI=1")
	cmd.Stdin = nil // agents must never wait for interactive input
	cmd.Stdout = outputFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 5 * time.Second

	runErr := cmd.Run()
	if _, err := outputFile.Seek(0, io.SeekStart); err != nil {
		return runResult{ExitCode: -1, Err: fmt.Errorf("rewind output: %w", err)}
	}
	output, readErr := io.ReadAll(outputFile)
	if readErr != nil {
		return runResult{ExitCode: -1, Err: fmt.Errorf("read output: %w", readErr)}
	}
	result := runResult{Output: string(output)}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	switch {
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		result.Err = fmt.Errorf("timed out after %s", timeout)
	case errors.Is(ctx.Err(), context.Canceled):
		result.Err = errors.New("canceled")
	case runErr != nil:
		result.Err = fmt.Errorf("agent exited: %w", runErr)
	}
	return result
}
