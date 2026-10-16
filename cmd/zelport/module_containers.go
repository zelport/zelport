//go:build containers

package main

import "github.com/zelport/zelport/internal/modules/containers"

func init() { factories = append(factories, containers.New) }
