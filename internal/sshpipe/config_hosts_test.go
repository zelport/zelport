package sshpipe

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestHostAliasesListsConcreteHostsFromConfigAndIncludes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	sshDir := filepath.Join(root, ".ssh")
	if err := os.Mkdir(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config")
	if err := os.Mkdir(filepath.Join(sshDir, "conf.d"), 0700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		config: "Host office-server tv.example.test *.lan !blocked\n  HostName example.test\nInclude \"" + filepath.Join(sshDir, "conf.d", "*.conf") + "\"\nHost office-server\n",
		filepath.Join(sshDir, "conf.d", "extra.conf"): "Host git-server\nHost *\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := HostAliases(config)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"office-server", "tv.example.test", "git-server"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HostAliases() = %q, want %q", got, want)
	}
}

func TestHostAliasesIncludeQuotedFileWithSpaces(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	include := filepath.Join(root, "included = hosts")
	if err := os.WriteFile(include, []byte("Host included-host\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("Include \""+include+"\"\nHost root-host\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := HostAliases(config)
	if err != nil || !reflect.DeepEqual(got, []string{"included-host", "root-host"}) {
		t.Fatalf("HostAliases() = %q, %v", got, err)
	}
}

func TestHostAliasesRelativeIncludesMatchOpenSSH(t *testing.T) {
	binary, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("SSH client unavailable")
	}
	root := t.TempDir()
	// Relative Includes are anchored under ~/.ssh, rather than the selected
	// file's directory. All fixtures stay in a temporary HOME.
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	if err := os.Mkdir(filepath.Join(root, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	relative := "zelport-include-" + filepath.Base(root)
	include := filepath.Join(root, ".ssh", relative)
	if err := os.WriteFile(include, []byte("Host included-host\n User included-user\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, relative), []byte("Host decoy-host\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config")
	if err := os.WriteFile(config, []byte("Include \""+relative+"\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(binary, "-G", "-F", config, "included-host").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "user included-user\n") {
		t.Fatalf("OpenSSH fixture: %v: %s", err, out)
	}
	got, err := HostAliases(config)
	if err != nil || !reflect.DeepEqual(got, []string{"included-host"}) {
		t.Fatalf("HostAliases() = %q, %v", got, err)
	}
}

func TestHostAliasesMissingConfigIsEmpty(t *testing.T) {
	got, err := HostAliases(filepath.Join(t.TempDir(), "missing"))
	if err != nil || len(got) != 0 {
		t.Fatalf("HostAliases() = %q, %v; want empty list", got, err)
	}
}

func TestHostAliasesIncludeHashInFilenameMatchesOpenSSH(t *testing.T) {
	binary, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("SSH client unavailable")
	}
	root := t.TempDir()
	include := filepath.Join(root, "team#hosts")
	if err := os.WriteFile(include, []byte("Host selected-host\n User selected-user\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config")
	if err := os.WriteFile(config, []byte("Include "+include+" # ignored.conf\nHost root-host # ignored-host\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(binary, "-G", "-F", config, "selected-host").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "user selected-user\n") {
		t.Fatalf("OpenSSH fixture: %v: %s", err, out)
	}
	got, err := HostAliases(config)
	if err != nil || !reflect.DeepEqual(got, []string{"selected-host", "root-host"}) {
		t.Fatalf("HostAliases() = %q, %v", got, err)
	}
}

func TestHostAliasesIncludeEscapedWildcardMatchesOpenSSH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows forbids wildcard characters in filenames")
	}
	binary, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("SSH client unavailable")
	}
	root := t.TempDir()
	include := filepath.Join(root, "a*b")
	if err := os.WriteFile(include, []byte("Host selected-host\n User selected-user\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "axb"), []byte("Host decoy-host\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config")
	for _, escaped := range []string{`\*`, `\\*`} {
		t.Run(escaped, func(t *testing.T) {
			if err := os.WriteFile(config, []byte("Include \""+strings.Replace(include, "*", escaped, 1)+"\"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(binary, "-G", "-F", config, "selected-host").CombinedOutput()
			if err != nil || !strings.Contains(string(out), "user selected-user\n") {
				t.Fatalf("OpenSSH fixture: %v: %s", err, out)
			}
			got, err := HostAliases(config)
			if err != nil || !reflect.DeepEqual(got, []string{"selected-host"}) {
				t.Fatalf("HostAliases() = %q, %v", got, err)
			}
		})
	}
}
