//go:build windows

package review

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// killTreeOnCancel stops the agent and every process it spawned; Windows has
// no process groups, so taskkill walks the tree.
func killTreeOnCancel(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}

// agentCandidates tries the PATHEXT extensions (npm installs agents as .cmd
// shims) unless binary already has one.
func agentCandidates(dir, binary string) []string {
	if filepath.Ext(binary) != "" {
		return []string{filepath.Join(dir, binary)}
	}
	extensions := os.Getenv("PATHEXT")
	if extensions == "" {
		extensions = ".COM;.EXE;.BAT;.CMD"
	}
	candidates := []string{}
	for _, extension := range strings.Split(extensions, ";") {
		if extension != "" {
			candidates = append(candidates, filepath.Join(dir, binary+strings.ToLower(extension)))
		}
	}
	return candidates
}

func isRunnable(info fs.FileInfo) bool {
	return !info.IsDir()
}
