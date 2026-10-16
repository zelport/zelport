//go:build built_in_relay

package main

import "github.com/zelport/zelport/internal/modules/built_in_relay"

func init() { factories = append(factories, relay.New) }
