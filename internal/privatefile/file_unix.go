//go:build !windows

// Package privatefile creates private temporary files for configuration and state.
package privatefile

import "os"

func CreateTemp(dir string) (*os.File, error) { return os.CreateTemp(dir, ".zelport-*") }
func Replace(from, to string) error           { return os.Rename(from, to) }
