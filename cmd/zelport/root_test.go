package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootPolicy(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		uid                   int
		allow, denied, warned bool
	}{
		{"root rejected", 0, false, true, false}, {"root allowed", 0, true, false, true},
		{"regular user", 501, false, false, false}, {"unnecessary override", 501, true, false, false},
		{"unsupported UID", -1, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := checkRoot(tc.uid, tc.allow, &out)
			if (err != nil) != tc.denied {
				t.Fatalf("error: %v", err)
			}
			if err != nil && (!strings.Contains(err.Error(), "--allow-root") || !strings.Contains(err.Error(), allowRootEnv+"=1")) {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "WARNING:") != tc.warned {
				t.Fatalf("warning: %q", out.String())
			}
		})
	}
}
