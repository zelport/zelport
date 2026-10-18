package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestVersionExitsBeforeStartup(t *testing.T) {
	oldUID := effectiveUID
	effectiveUID = func() int { return 0 }
	defer func() { effectiveUID = oldUID }()
	oldArgs, oldFlags, oldOut, oldVersion := os.Args, flag.CommandLine, os.Stdout, version
	defer func() { os.Args, flag.CommandLine, os.Stdout, version = oldArgs, oldFlags, oldOut, oldVersion }()
	for _, v := range []string{"dev", "0.1.0"} {
		t.Run(v, func(t *testing.T) {
			version = v
			// Neither the invalid port nor missing configuration should be used.
			os.Args = []string{"zelport", "--version", "--port=0", "--config=" + filepath.Join(t.TempDir(), "missing.json")}
			flag.CommandLine = flag.NewFlagSet("zelport", flag.ContinueOnError)
			out, err := os.CreateTemp(t.TempDir(), "stdout")
			if err != nil {
				t.Fatal(err)
			}
			defer out.Close()
			os.Stdout = out
			if err := run(); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(out.Name())
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "zelport "+v+" (built "+builtAt+")\n" {
				t.Fatalf("output %q", got)
			}
		})
	}
}

func TestRootStartupGuard(t *testing.T) {
	oldArgs, oldFlags, oldUID := os.Args, flag.CommandLine, effectiveUID
	defer func() { os.Args, flag.CommandLine, effectiveUID = oldArgs, oldFlags, oldUID }()
	effectiveUID = func() int { return 0 }
	for _, tc := range []struct {
		name, env string
		args      []string
		want      string
	}{
		{"default", "", nil, "Refusing to run as root for security reasons."},
		{"env", "1", nil, "port must be"},
		{"not truthy", "true", nil, "Refusing to run as root for security reasons."},
		{"flag", "", []string{"--allow-root"}, "port must be"},
		{"false overrides env", "1", []string{"--allow-root=false"}, "Refusing to run as root for security reasons."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(allowRootEnv, tc.env)
			os.Args = append([]string{"zelport", "--port=0"}, tc.args...)
			flag.CommandLine = flag.NewFlagSet("zelport", flag.ContinueOnError)
			err := run()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestRootHelp(t *testing.T) {
	oldArgs, oldFlags, oldUID := os.Args, flag.CommandLine, effectiveUID
	defer func() { os.Args, flag.CommandLine, effectiveUID = oldArgs, oldFlags, oldUID }()
	effectiveUID = func() int { t.Fatal("help checked process identity"); return 0 }
	os.Args = []string{"zelport", "--help"}
	flag.CommandLine = flag.NewFlagSet("zelport", flag.ContinueOnError)
	var out bytes.Buffer
	flag.CommandLine.SetOutput(&out)
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "allow-root") || !strings.Contains(out.String(), allowRootEnv+"=1") {
		t.Fatal(out.String())
	}
}

func TestStartupLogsUseProductName(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	oldArgs, oldFlags, oldUID, oldVersion, oldBuiltAt := os.Args, flag.CommandLine, effectiveUID, version, builtAt
	oldLogOutput := log.Writer()
	defer func() {
		os.Args, flag.CommandLine, effectiveUID, version, builtAt = oldArgs, oldFlags, oldUID, oldVersion, oldBuiltAt
		log.SetOutput(oldLogOutput)
	}()
	effectiveUID = func() int { return 1000 }
	version = "0.10.0"
	builtAt = "2026-10-18 10:03:04 UTC"
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("LOCALAPPDATA", stateDir)
	os.Args = []string{"zelport", "--port=" + strconv.Itoa(port), "--config=" + filepath.Join(t.TempDir(), "config.json")}
	flag.CommandLine = flag.NewFlagSet("zelport", flag.ContinueOnError)
	output, err := os.CreateTemp(t.TempDir(), "startup-log")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	log.SetOutput(output)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- runWithRuntime(func() (context.Context, context.CancelFunc) {
			return ctx, cancel
		}, func(requestedPort int) ([]net.Listener, error) {
			if requestedPort != port {
				return nil, fmt.Errorf("listen port %d, want %d", requestedPort, port)
			}
			return []net.Listener{listener}, nil
		})
	}()
	finished := false
	defer func() {
		if finished {
			return
		}
		cancel()
		select {
		case <-result:
		case <-time.After(5 * time.Second):
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	var body string
	for {
		select {
		case err := <-result:
			finished = true
			t.Fatalf("Zelport App exited before startup: %v\n%s", err, body)
		default:
		}
		content, err := os.ReadFile(output.Name())
		if err != nil {
			t.Fatal(err)
		}
		body = string(content)
		if strings.Contains(body, "Configuration: ") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("startup timed out:\n%s", body)
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-result:
		finished = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Zelport App ignored cancellation")
	}
	content, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	body = string(content)
	for _, want := range []string{
		"Zelport App 0.10.0",
		fmt.Sprintf("Zelport App is listening at http://localhost:%d (loopback only)", port),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("startup log does not contain %q:\n%s", want, body)
		}
	}
}
