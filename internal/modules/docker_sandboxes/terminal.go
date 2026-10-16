package docker_sandboxes

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Shell arguments are quoted independently in the temporary launcher.
// AppleScript receives only the profile path as data, never source code.
func terminalCommand(binary, name string) string {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	return quote(binary) + " exec -it " + quote(name) + " bash"
}

// A dedicated window profile replaces the login shell rather than typing a
// command into it. XML and shell quoting are separate encoding boundaries.
func terminalProfileCommand(value string) []byte {
	var command bytes.Buffer
	_ = xml.EscapeText(&command, []byte(value))
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>name</key><string>Zelport</string>
<key>type</key><string>Window Settings</string>
<key>ProfileCurrentVersion</key><real>2.07</real>
<key>RunCommandAsShell</key><true/>
<key>CommandString</key><string>` + command.String() + `</string>
</dict></plist>`)
}

func openTerminal(ctx context.Context, name string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("opening Terminal.app is supported only on macOS")
	}
	binary, err := exec.LookPath("sbx")
	if err != nil {
		return fmt.Errorf("find sbx: %w", err)
	}
	dir, err := os.MkdirTemp("/tmp", "zelport-terminal-")
	if err != nil {
		return err
	}
	// The launcher removes its files after Terminal has actually started it.
	profile := filepath.Join(dir, "Zelport.terminal")
	launcher := filepath.Join(dir, "launch")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	source := "#!/bin/sh\n" + "export PATH=" + quote(os.Getenv("PATH")) + "\n" + "/bin/rm -f " + quote(profile) + " " + quote(launcher) + "\n/bin/rmdir " + quote(dir) + "\nexec " + terminalCommand(binary, name) + "\n"
	if err := os.WriteFile(launcher, []byte(source), 0700); err != nil {
		os.RemoveAll(dir)
		return err
	}
	if err := os.WriteFile(profile, terminalProfileCommand(launcher), 0600); err != nil {
		os.RemoveAll(dir)
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	const script = `on run argv
 set profileFile to POSIX file (item 1 of argv) as alias
 tell application "Terminal"
  open profileFile
  activate
 end tell
end run`
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script, profile)
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(dir)
		return fmt.Errorf("open Terminal.app: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
