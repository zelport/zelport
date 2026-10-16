package docker_sandboxes

import (
	"encoding/xml"
	"os/exec"
	"strings"
	"testing"
)

func TestTerminalCommandQuoting(t *testing.T) {
	for _, name := range []string{"codex-project", "a'\";$(echo injected)`echo bad`\\\nname"} {
		// printf replaces sbx to observe exactly the arguments delivered by the shell.
		cmd := terminalCommand("/usr/bin/printf", name)
		cmd = "set -- " + cmd + "; printf '%s' \"$4\""
		out, err := exec.Command("/bin/sh", "-c", cmd).Output()
		if err != nil || string(out) != name {
			t.Fatalf("round trip: %q, %v", out, err)
		}
	}
}

func TestTerminalProfile(t *testing.T) {
	name := "project<&\"'"
	profile := terminalProfileCommand(terminalCommand("/opt/homebrew/bin/sbx", name))
	var plist struct {
		Dict struct {
			Keys    []string   `xml:"key"`
			Strings []string   `xml:"string"`
			True    []struct{} `xml:"true"`
		} `xml:"dict"`
	}
	if err := xml.Unmarshal(profile, &plist); err != nil {
		t.Fatal(err)
	}
	if len(plist.Dict.True) != 1 || !strings.Contains(string(profile), "<key>RunCommandAsShell</key><true/>") {
		t.Fatal("login shell was not replaced")
	}
	if got := plist.Dict.Strings[len(plist.Dict.Strings)-1]; got != terminalCommand("/opt/homebrew/bin/sbx", name) {
		t.Fatalf("command changed across XML: %q", got)
	}
}
