package sshpipe

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHostAliasesListsConcreteHostsFromConfigAndIncludes(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config")
	if err := os.Mkdir(filepath.Join(root, "conf.d"), 0700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		config: "Host office-server tv.slovan.lan *.lan !blocked\n  HostName example.test\nInclude conf.d/*.conf\nHost office-server\n",
		filepath.Join(root, "conf.d", "extra.conf"): "Host git-server\nHost *\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := HostAliases(config)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"office-server", "tv.slovan.lan", "git-server"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HostAliases() = %q, want %q", got, want)
	}
}

func TestHostAliasesMissingConfigIsEmpty(t *testing.T) {
	got, err := HostAliases(filepath.Join(t.TempDir(), "missing"))
	if err != nil || len(got) != 0 {
		t.Fatalf("HostAliases() = %q, %v; want empty list", got, err)
	}
}
