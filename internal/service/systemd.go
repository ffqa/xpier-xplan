package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// systemdUnit is the user unit name.
const systemdUnit = "xplan.service"

type systemd struct {
	userHome string
}

func (s *systemd) Name() string { return "systemd" }

func (s *systemd) UnitFile() string {
	return filepath.Join(s.userHome, ".config", "systemd", "user", systemdUnit)
}

func (s *systemd) Install(spec Spec) error {
	if err := ensureLogDir(spec); err != nil {
		return err
	}
	if err := writeUnit(s.UnitFile(), systemdUnitFile(spec)); err != nil {
		return err
	}
	if _, err := run("systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	if _, err := run("systemctl", "--user", "enable", systemdUnit); err != nil {
		return err
	}
	_, err := run("systemctl", "--user", "restart", systemdUnit)
	return err
}

func (s *systemd) Uninstall() error {
	if fileExists(s.UnitFile()) {
		if _, err := run("systemctl", "--user", "disable", "--now", systemdUnit); err != nil {
			return err
		}
	}
	if err := os.Remove(s.UnitFile()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", s.UnitFile(), err)
	}
	_, err := run("systemctl", "--user", "daemon-reload")
	return err
}

func (s *systemd) Restart() error {
	_, err := run("systemctl", "--user", "restart", systemdUnit)
	return err
}

func (s *systemd) Status() (Status, error) {
	status := Status{Manager: s.Name(), UnitFile: s.UnitFile(), Installed: fileExists(s.UnitFile())}
	output, err := run("systemctl", "--user", "show", systemdUnit, "--property=ActiveState,MainPID")
	if err != nil {
		return status, err
	}
	for _, line := range strings.Split(output, "\n") {
		key, value, _ := strings.Cut(strings.TrimSpace(line), "=")
		switch key {
		case "ActiveState":
			status.Running = value == "active"
		case "MainPID":
			status.PID, _ = strconv.Atoi(value)
		}
	}
	return status, nil
}

// systemdUnitFile renders the user unit. Restart=on-failure mirrors launchd's
// KeepAlive: a crash restarts xplan, a deliberate stop does not.
func systemdUnitFile(spec Spec) string {
	quoted := make([]string, 0, 6)
	for _, arg := range serveArgs(spec) {
		quoted = append(quoted, systemdQuote(arg))
	}
	crashLog := strings.TrimSuffix(spec.LogFile, ".log") + ".crash.log"
	return fmt.Sprintf(`[Unit]
Description=xplan, cockpit for parallel projects
After=network.target

[Service]
ExecStart=%s
WorkingDirectory=%s
Environment=%s
Environment=XPLAN_SERVICE=systemd
Restart=on-failure
RestartSec=10
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, strings.Join(quoted, " "), systemdQuote(spec.Home), systemdQuote("PATH="+spec.Path), crashLog, crashLog)
}

// systemdQuote quotes one word for a unit file: double quotes, with
// backslashes, quotes and % specifiers escaped.
func systemdQuote(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%").Replace(value)
	return `"` + escaped + `"`
}
