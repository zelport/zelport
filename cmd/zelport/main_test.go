package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
