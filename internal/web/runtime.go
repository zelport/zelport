package web

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// RuntimeInfo describes this process; it contains no configuration contents.
type RuntimeInfo struct {
	Version           string    `json:"version"`
	BuiltAt           string    `json:"built_at"`
	StartedAt         time.Time `json:"started_at"`
	UptimeSeconds     int64     `json:"uptime_seconds"`
	PID               int       `json:"pid"`
	GoVersion         string    `json:"go_version"`
	Platform          string    `json:"platform"`
	Hostname          string    `json:"hostname"`
	ConfigPath        string    `json:"config_path"`
	ConfigDisplayPath string    `json:"config_display_path"`
	Listeners         []string  `json:"listeners"`
}

func newRuntimeInfo(options []RuntimeInfo) RuntimeInfo {
	var info RuntimeInfo
	if len(options) > 0 {
		info = options[0]
	}
	if info.StartedAt.IsZero() {
		info.StartedAt = time.Now()
	}
	if info.Version == "" {
		info.Version = "dev"
	}
	if info.BuiltAt == "" {
		info.BuiltAt = "unknown"
	}
	if info.Hostname == "" {
		info.Hostname, _ = os.Hostname()
	}
	if info.Hostname == "" {
		info.Hostname = "unknown"
	}
	home, _ := os.UserHomeDir()
	info.ConfigDisplayPath = displayConfigPath(info.ConfigPath, home)
	info.PID = os.Getpid()
	info.GoVersion = runtime.Version()
	info.Platform = runtime.GOOS + "/" + runtime.GOARCH
	info.Listeners = append([]string(nil), info.Listeners...)
	return info
}
func (info RuntimeInfo) current() RuntimeInfo {
	info.UptimeSeconds = max(0, int64(time.Since(info.StartedAt).Seconds()))
	return info
}

// Shorten only paths inside this process user's home, on a path-component boundary.
func displayConfigPath(path, home string) string {
	if !filepath.IsAbs(path) || !filepath.IsAbs(home) {
		return path
	}
	relative, err := filepath.Rel(home, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return path
	}
	if relative == "." {
		return "~"
	}
	return "~" + string(filepath.Separator) + relative
}
