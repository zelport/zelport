package sshpipe

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func Arguments(host string, port int) []string {
	args := []string{"-T", "-a", "-x"}
	for _, option := range []string{
		"BatchMode=yes", "StrictHostKeyChecking=yes", "UpdateHostKeys=no",
		"ConnectTimeout=10", "ConnectionAttempts=1", "ServerAliveInterval=15", "ServerAliveCountMax=2",
		"ExitOnForwardFailure=yes", "ClearAllForwardings=yes", "PermitLocalCommand=no",
		"ForkAfterAuthentication=no", "StdinNull=no", "RemoteCommand=none",
		"ControlMaster=no", "ControlPath=none", "ControlPersist=no", "LogLevel=ERROR",
	} {
		args = append(args, "-o", option)
	}
	return append(args, "-W", "127.0.0.1:"+strconv.Itoa(port), "--", host)
}

// The same -F file is inherited by OpenSSH's implicit ProxyJump children.
// Only references are stored. User keys and configuration are never copied.
func writeConfig() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(home, "\r\n") {
		return "", errors.New("invalid home directory")
	}
	quote := func(s string) string { return "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(s) + "\"" }
	f, err := os.CreateTemp("", "zelport-ssh-*.conf")
	if err != nil {
		return "", err
	}
	content := "Host *\n"
	args := Arguments("unused", 1)
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-o" {
			content += "  " + strings.Replace(args[i+1], "=", " ", 1) + "\n"
			i++
		}
	}
	content += "  RequestTTY no\n  ForwardAgent no\n  ForwardX11 no\n"
	content += "Include " + quote(filepath.Join(home, ".ssh", "config")) + "\nHost *\nInclude /etc/ssh/ssh_config\n"
	_, err = f.WriteString(content)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}
