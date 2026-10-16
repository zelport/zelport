//go:build docker_sandboxes

package main

import "github.com/zelport/zelport/internal/modules/docker_sandboxes"

func init() { factories = append(factories, docker_sandboxes.New) }
