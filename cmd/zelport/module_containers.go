//go:build !zelport_core || containers

package main

import "github.com/zelport/zelport/internal/modules/containers"

func init() { factories = append(factories, containers.New) }
