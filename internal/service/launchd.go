package service

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// launchdLabel names the LaunchAgent; it is also the plist file name.
const launchdLabel = "com.ffqa.xplan"

type launchd struct {
	userHome string
	uid      int
}

func (l *launchd) Name() string { return "launchd" }

func (l *launchd) UnitFile() string {
	return filepath.Join(l.userHome, "Library", "LaunchAgents", launchdLabel+".plist")
}

func (l *launchd) domain() string { return fmt.Sprintf("gui/%d", l.uid) }
func (l *launchd) target() string { return l.domain() + "/" + launchdLabel }
func (l *launchd) loaded() bool   { _, err := run("launchctl", "print", l.target()); return err == nil }
func (l *launchd) bootout() error { _, err := run("launchctl", "bootout", l.target()); return err }
func (l *launchd) Restart() error {
	_, err := run("launchctl", "kickstart", "-k", l.target())
	return err
}
func (l *launchd) bootstrap() error {
	_, err := run("launchctl", "bootstrap", l.domain(), l.UnitFile())
	return err
}

func (l *launchd) Install(spec Spec) error {
	if err := ensureLogDir(spec); err != nil {
		return err
	}
	if l.loaded() {
		if err := l.bootout(); err != nil {
			return err
		}
		WaitForPortFree(spec.Listen, 10*time.Second)
	}
	if err := writeUnit(l.UnitFile(), launchdPlist(spec)); err != nil {
		return err
	}
	return l.bootstrap()
}

func (l *launchd) Uninstall() error {
	if l.loaded() {
		if err := l.bootout(); err != nil {
			return err
		}
	}
	if err := os.Remove(l.UnitFile()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", l.UnitFile(), err)
	}
	return nil
}

var (
	launchdState = regexp.MustCompile(`(?m)^\s*state = (\S+)`)
	launchdPID   = regexp.MustCompile(`(?m)^\s*pid = (\d+)`)
)

func (l *launchd) Status() (Status, error) {
	status := Status{Manager: l.Name(), UnitFile: l.UnitFile(), Installed: fileExists(l.UnitFile())}
	output, err := run("launchctl", "print", l.target())
	if err != nil {
		return status, nil // not loaded
	}
	if match := launchdState.FindStringSubmatch(output); match != nil {
		status.Running = match[1] == "running"
	}
	if match := launchdPID.FindStringSubmatch(output); match != nil {
		status.PID, _ = strconv.Atoi(match[1])
	}
	return status, nil
}

// launchdPlist renders the LaunchAgent. KeepAlive restarts xplan only after a
// crash, so a deliberate stop stays stopped; the server writes its own
// rotating log, and launchd's stdout/stderr files only catch crash output.
func launchdPlist(spec Spec) string {
	crashLog := strings.TrimSuffix(spec.LogFile, ".log") + ".crash.log"
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	writeKeyString(&b, "Label", launchdLabel)
	b.WriteString("  <key>ProgramArguments</key>\n  <array>\n")
	for _, arg := range serveArgs(spec) {
		fmt.Fprintf(&b, "    <string>%s</string>\n", xmlEscape(arg))
	}
	b.WriteString("  </array>\n")
	b.WriteString("  <key>EnvironmentVariables</key>\n  <dict>\n")
	fmt.Fprintf(&b, "    <key>PATH</key>\n    <string>%s</string>\n", xmlEscape(spec.Path))
	b.WriteString("    <key>XPLAN_SERVICE</key>\n    <string>launchd</string>\n")
	b.WriteString("  </dict>\n")
	writeKeyString(&b, "WorkingDirectory", spec.Home)
	b.WriteString("  <key>RunAtLoad</key>\n  <true/>\n")
	b.WriteString("  <key>KeepAlive</key>\n  <dict>\n    <key>SuccessfulExit</key>\n    <false/>\n  </dict>\n")
	b.WriteString("  <key>ThrottleInterval</key>\n  <integer>10</integer>\n")
	writeKeyString(&b, "StandardOutPath", crashLog)
	writeKeyString(&b, "StandardErrorPath", crashLog)
	b.WriteString("</dict>\n</plist>\n")
	return b.String()
}

// serveArgs is the command line every service manager starts.
func serveArgs(spec Spec) []string {
	return []string{spec.Binary, "--home", spec.Home, "serve", "--log-file", spec.LogFile}
}

func writeKeyString(b *strings.Builder, key, value string) {
	fmt.Fprintf(b, "  <key>%s</key>\n  <string>%s</string>\n", key, xmlEscape(value))
}

func xmlEscape(value string) string {
	var buffer bytes.Buffer
	_ = xml.EscapeText(&buffer, []byte(value))
	return buffer.String()
}
