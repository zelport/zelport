//go:build !zelport_core || local_ports

package main

import "github.com/zelport/zelport/internal/modules/local_ports"

func init() { factories = append(factories, local_ports.New) }
