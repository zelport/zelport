package main

import (
	"fmt"
	"io"
	"os"
)

const allowRootEnv = "ZELPORT_ALLOW_ROOT"

var effectiveUID = os.Geteuid

// This is an accidental-startup guard, not a security boundary against root.
func checkRoot(uid int, allow bool, warnings io.Writer) error {
	if uid != 0 {
		return nil
	}
	if !allow {
		return fmt.Errorf("Refusing to run as root for security reasons. Override with --allow-root or %s=1.", allowRootEnv)
	}
	fmt.Fprintln(warnings, "WARNING: Running Zelport as root. Configuration, SSH credentials, and container access may differ from those of your regular user.")
	return nil
}
