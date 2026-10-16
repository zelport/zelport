package sbx

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
)

// CLIInfo describes the executable, not sandbox or application availability.
type CLIInfo struct {
	Status  string `json:"status"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (c Client) Info(ctx context.Context) CLIInfo {
	lookup := c.LookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	path, err := lookup("sbx")
	if err != nil {
		return CLIInfo{Status: "not_found", Error: err.Error()}
	}
	info := CLIInfo{Status: "found", Path: path}
	raw, err := c.call(ctx, "version", "--json")
	if err != nil {
		info.Error = err.Error()
		return info
	}
	var output struct {
		Client struct {
			Version string `json:"version"`
		} `json:"client"`
	}
	if err = json.Unmarshal(raw, &output); err != nil || strings.TrimSpace(output.Client.Version) == "" {
		info.Error = "Cannot read sbx version from CLI output"
		return info
	}
	info.Version = strings.TrimSpace(output.Client.Version)
	return info
}
