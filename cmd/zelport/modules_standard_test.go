//go:build !zelport_core

package main

import (
	"slices"
	"testing"
)

func TestStandardBuildContainsSupportedModules(t *testing.T) {
	got := make([]string, 0, len(factories))
	for _, factory := range factories {
		got = append(got, factory().Descriptor().ID)
	}
	slices.Sort(got)

	want := []string{
		"built_in_relay",
		"local_ports",
		"ports_over_relay",
		"ports_over_ssh",
		"remote_http",
	}
	if len(got) != len(want) {
		t.Fatalf("standard build module count = %d, want %d; IDs = %v", len(got), len(want), got)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("standard build module IDs = %v, want %v", got, want)
	}
}
