//go:build remote_http

package main

import "github.com/zelport/zelport/internal/modules/remote_http"

func init() { factories = append(factories, remote_http.New) }
