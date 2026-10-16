//go:build darwin || linux || freebsd || openbsd || netbsd

package sshpipe

import (
	"os/exec"
	"syscall"
)

func prepareProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
func stopProcess(cmd *exec.Cmd)    { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
