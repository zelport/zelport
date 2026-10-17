//go:build !zelport_core || ports_over_relay

package main

import "github.com/zelport/zelport/internal/modules/ports_over_relay"

func init() { factories = append(factories, ports_over_relay.New) }
