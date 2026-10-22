package sshpipe

import "testing"

func TestDiagnosticStripsANSIEscapeSequences(t *testing.T) {
	var d diagnosticBuffer
	_, _ = d.Write([]byte("\x1b[0;1;37m17:18:14 \x1b[0m\x1b[0;1;31mfatal \x1b[0mworkspace doesn't exist\n"))
	if got, want := d.String(), "17:18:14 fatal workspace doesn't exist"; got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
}
