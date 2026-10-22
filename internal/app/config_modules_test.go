package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnknownModuleConfigurationRejected(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "module settings",
			raw:  `{"version":1,"revision":4,"modules":{"missing":{}},"routes":[]}`,
		},
		{
			name: "route owner",
			raw:  `{"version":1,"revision":4,"modules":{},"routes":[{"id":"b60d7e28-d39e-4ad4-9a36-50f9158f35cb","module":"missing","domain":"saved.localhost","settings":{}}]}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			before := []byte(test.raw + "\n")
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			manager, err := New(path, 8080, &testModule{id: "present"})
			if err == nil {
				manager.Close()
				t.Fatal("accepted unknown module configuration")
			}
			if !strings.Contains(err.Error(), `module "missing" is not included in this build`) {
				t.Fatalf("error = %q", err)
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(after) != string(before) {
				t.Fatalf("configuration changed\nbefore: %s\nafter: %s", before, after)
			}
		})
	}
}
