package containers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type nerdctlInfo struct {
	Namespace string
	Daemon    struct{ Server struct{ UUID string } }
}

func (c Client) nerdctlInfo(ctx context.Context, prefix []string) (nerdctlInfo, error) {
	var info nerdctlInfo
	data, err := c.call(ctx, append(append([]string{}, prefix...), "info", "--mode", "native", "--format", "{{json .}}")...)
	if err != nil {
		return info, err
	}
	if err = json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("invalid nerdctl runtime info JSON: %w", err)
	}
	if info.Namespace == "" || strings.HasPrefix(info.Namespace, "-") || info.Daemon.Server.UUID == "" {
		return info, errors.New("incomplete nerdctl runtime identity")
	}
	return info, nil
}
func (c Client) nerdctl(ctx context.Context, inv *Inventory) error {
	// VM wrappers report guest ports. They cannot establish host reachability.
	if c.platform() != "linux" {
		return errors.New("nerdctl discovery currently requires native Linux; VM wrapper port forwarding is not verified")
	}
	inv.RuntimeStatus = "unavailable"
	info, err := c.nerdctlInfo(ctx, nil)
	if err != nil {
		return err
	}
	inv.RuntimeStatus = "available"
	inv.Namespace = info.Namespace
	inv.Label = info.Namespace
	inv.Connection = fingerprint(c.Engine, info.Namespace, info.Daemon.Server.UUID)
	prefix := []string{"--namespace", info.Namespace}
	if err = c.inspectContainers(ctx, prefix, inv); err != nil {
		return err
	}
	// Recheck the server identity: a changed CLI config must never mix inventories.
	after, err := c.nerdctlInfo(ctx, prefix)
	if err != nil {
		return err
	}
	if after != info {
		return errors.New("containerd connection changed during discovery")
	}
	return nil
}
