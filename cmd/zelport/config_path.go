package main

import "github.com/zelport/zelport/internal/userpaths"

func resolveConfigPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	return userpaths.Config()
}
