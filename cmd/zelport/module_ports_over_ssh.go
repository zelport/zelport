//go:build !zelport_core || ports_over_ssh

package main

import "github.com/zelport/zelport/internal/modules/ports_over_ssh"

func init() { factories = append(factories, ports_over_ssh.New) }
