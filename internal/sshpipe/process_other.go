//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd

package sshpipe

import "os/exec"

func prepareProcess(cmd *exec.Cmd) {}
func stopProcess(cmd *exec.Cmd)    { _ = cmd.Process.Kill() }
