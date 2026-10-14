//go:build !windows

package review

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"syscall"
)

// killTreeOnCancel starts the agent in its own process group so that cancel
// and timeout also stop the tools it spawned.
func killTreeOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// agentCandidates lists the files that would run binary from dir.
func agentCandidates(dir, binary string) []string {
	return []string{filepath.Join(dir, binary)}
}

func isRunnable(info fs.FileInfo) bool {
	return !info.IsDir() && info.Mode()&0o111 != 0
}
