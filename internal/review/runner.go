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
	"github.com/ffqa/xpier-xplan/internal/i18n"
)

// runResult is what one agent invocation produced.
type runResult struct {
	Output   string
	ExitCode int
	Err      error
}

// ErrAgentMissing means the reviewer's binary is not on PATH.
var ErrAgentMissing = errors.New("agent is not installed")

// errTimedOut marks runs stopped by the review deadline.
var errTimedOut = errors.New("timed out")

// timeoutError is a run stopped by the deadline; it matches errTimedOut.
type timeoutError struct{ after time.Duration }

func (e timeoutError) Error() string        { return i18n.T("timed out after %s", e.after) }
func (e timeoutError) Is(target error) bool { return target == errTimedOut }

// errNoOutput marks runs that exited cleanly without printing anything; some
// CLIs do that when they are not logged in.
var errNoOutput = errors.New("the agent exited without printing anything")

// loginError is a run that exited cleanly but only printed a login problem.
type loginError struct{ line string }

func (e loginError) Error() string { return i18n.T("the agent asked for a login: %s", e.line) }

// shortAnswer is the output length below which a clean exit that mentions a
// login problem counts as a failure; real reports are far longer.
const shortAnswer = 800

// explainRunError is the job error people see for a failed run.
func explainRunError(err error) string {
	var timeout timeoutError
	switch {
	case errors.As(err, &timeout):
		return i18n.T("No answer within %s. The agent's model service may be down or rate-limited (many agents retry quietly), or the project needs more time. Try another model or reviewer, or give this reviewer a longer timeout in Settings → Reviewer agents.", timeout.after)
	case errors.Is(err, errNoOutput):
		return i18n.T("The agent exited without printing anything. It may not be logged in: run it once in a terminal.")
	}
	return err.Error()
}

// ArgvVars are the values substituted into a reviewer's argv template.
type ArgvVars struct {
	Prompt     string
	PromptFile string
	Path       string
	Home       string
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
			return "", fmt.Errorf("%w: %s not found", ErrAgentMissing, binary)
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
	return "", fmt.Errorf("%w: %q is not on PATH", ErrAgentMissing, binary)
}

// BuildArgv substitutes the placeholders in a reviewer template. Arguments
// with an empty {model} or {agent} are dropped. {prompt} is replaced last so
// prompt text can never inject another placeholder.
func BuildArgv(def config.ReviewerDef, vars ArgvVars) ([]string, error) {
	if len(def.Command) == 0 {
		return nil, errors.New("reviewer has an empty command")
	}
	argv := make([]string, 0, len(def.Command))
	for _, part := range def.Command {
		if emptySlot(part, config.SlotModel, def.Model) || emptySlot(part, config.SlotAgent, def.Agent) {
			continue
		}
		replaced := strings.ReplaceAll(part, config.SlotPath, vars.Path)
		replaced = strings.ReplaceAll(replaced, config.SlotHome, vars.Home)
		replaced = strings.ReplaceAll(replaced, config.SlotPromptFile, vars.PromptFile)
		replaced = strings.ReplaceAll(replaced, config.SlotModel, def.Model)
		replaced = strings.ReplaceAll(replaced, config.SlotAgent, def.Agent)
		replaced = strings.ReplaceAll(replaced, config.SlotPrompt, vars.Prompt)
		argv = append(argv, replaced)
	}
	return argv, nil
}

func emptySlot(part, slot, value string) bool {
	return value == "" && strings.Contains(part, slot)
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
		result.Err = timeoutError{after: timeout}
	case errors.Is(ctx.Err(), context.Canceled):
		result.Err = errors.New("canceled")
	case runErr != nil:
		result.Err = fmt.Errorf("agent exited: %w", runErr)
	case strings.TrimSpace(result.Output) == "":
		result.Err = errNoOutput
	case len(result.Output) < shortAnswer:
		if line := firstMatchingLine(result.Output, authPattern); line != "" {
			result.Err = loginError{line: line}
		}
	}
	return result
}
