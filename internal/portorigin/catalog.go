// Package portorigin keeps read-only inventories used to attribute local TCP
// listeners without coupling feature modules to one another.
package portorigin

import (
	"context"
	"sync"
	"time"

	"github.com/zelport/zelport/internal/containers"
	"github.com/zelport/zelport/internal/sbx"
)

type Health string

const (
	Refreshing Health = "refreshing"
	Ready      Health = "ready"
	Stale      Health = "stale"
	Degraded   Health = "degraded"
)

type Snapshot struct {
	Containers []containers.Inventory
	Sandboxes  []sbx.Sandbox
	Health     Health
	Generation uint64
	UpdatedAt  time.Time
}

type ContainerProvider interface {
	Scan(context.Context) containers.Inventory
}

type SandboxProvider interface {
	Info(context.Context) sbx.CLIInfo
	List(context.Context) ([]sbx.Sandbox, error)
	Ports(context.Context, string) ([]sbx.Port, error)
}

type containerState struct {
	inventory containers.Inventory
	hasGood   bool
	failed    bool
	installed bool
}

type sandboxState struct {
	boxes     []sbx.Sandbox
	hasGood   bool
	failed    bool
	installed bool
}

type Catalog struct {
	mu                 sync.RWMutex
	containerProviders []ContainerProvider
	sandboxProvider    SandboxProvider
	containers         []containerState
	sandbox            sandboxState
	health             Health
	generation         uint64
	updatedAt          time.Time
	refreshing         bool
	refreshDone        chan struct{}
	trigger            chan struct{}
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	started            bool
	closed             bool
}

func NewCatalog(containerProviders []ContainerProvider, sandboxProvider SandboxProvider) *Catalog {
	providers := append([]ContainerProvider{}, containerProviders...)
	return &Catalog{
		containerProviders: providers,
		sandboxProvider:    sandboxProvider,
		containers:         make([]containerState, len(providers)),
		health:             Refreshing,
	}
}

func NewDefaultCatalog() *Catalog {
	return NewCatalog(
		[]ContainerProvider{
			containers.Client{Engine: "docker"},
			containers.Client{Engine: "podman"},
			containers.Client{Engine: "nerdctl"},
		},
		sbx.Client{},
	)
}

func (c *Catalog) Start(parent context.Context, interval time.Duration) {
	c.mu.Lock()
	if c.started || c.closed {
		c.mu.Unlock()
		return
	}
	if interval <= 0 {
		interval = 15 * time.Second
	}
	c.ctx, c.cancel = context.WithCancel(parent)
	c.trigger = make(chan struct{}, 1)
	c.started = true
	c.wg.Add(1)
	ctx := c.ctx
	trigger := c.trigger
	c.mu.Unlock()

	go c.run(ctx, trigger, interval)
	c.RequestRefresh()
}

func (c *Catalog) run(ctx context.Context, trigger <-chan struct{}, interval time.Duration) {
	defer c.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.finishCancelledRefresh()
			return
		case <-ticker.C:
			c.RequestRefresh()
		case <-trigger:
			c.refresh(ctx)
		}
	}
}

func (c *Catalog) RequestRefresh() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || !c.started {
		done := make(chan struct{})
		close(done)
		return done
	}
	if c.refreshing {
		return c.refreshDone
	}
	c.refreshing = true
	c.refreshDone = make(chan struct{})
	c.health = c.healthLocked()
	done := c.refreshDone
	select {
	case c.trigger <- struct{}{}:
	default:
	}
	return done
}

func (c *Catalog) refresh(ctx context.Context) {
	var wg sync.WaitGroup
	for i, provider := range c.containerProviders {
		wg.Add(1)
		go func(index int, provider ContainerProvider) {
			defer wg.Done()
			c.commitContainer(index, provider.Scan(ctx))
		}(i, provider)
	}
	if c.sandboxProvider != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			boxes, installed, err := scanSandboxes(ctx, c.sandboxProvider)
			c.commitSandboxes(boxes, installed, err)
		}()
	}
	wg.Wait()

	c.mu.Lock()
	if c.refreshing {
		c.generation++
		c.refreshing = false
		c.health = c.healthLocked()
		if c.updatedAt.IsZero() && c.health == Ready {
			c.updatedAt = time.Now()
		}
		close(c.refreshDone)
	}
	c.mu.Unlock()
}

func (c *Catalog) commitContainer(index int, next containers.Inventory) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := &c.containers[index]
	state.installed = next.CLIStatus != "not_found"
	state.failed = next.Stale && state.installed
	if !next.Stale {
		state.inventory = cloneInventory(next)
		state.hasGood = true
		if next.UpdatedAt.After(c.updatedAt) {
			c.updatedAt = next.UpdatedAt
		}
	} else if state.hasGood && (next.Connection == "" || next.Connection == state.inventory.Connection) {
		kept := cloneInventory(state.inventory)
		kept.CLIStatus = next.CLIStatus
		kept.CLIPath = next.CLIPath
		kept.RuntimeStatus = next.RuntimeStatus
		kept.Stale = true
		kept.Error = next.Error
		state.inventory = kept
	} else {
		state.inventory = cloneInventory(next)
	}
	c.health = c.healthLocked()
}

func scanSandboxes(ctx context.Context, provider SandboxProvider) ([]sbx.Sandbox, bool, error) {
	info := provider.Info(ctx)
	if info.Status == "not_found" {
		return []sbx.Sandbox{}, false, nil
	}
	if info.Error != "" {
		return nil, true, contextError(ctx, info.Error)
	}
	boxes, err := provider.List(ctx)
	if err != nil {
		return nil, true, err
	}
	for i := range boxes {
		if boxes[i].Status != "running" {
			continue
		}
		ports, err := provider.Ports(ctx, boxes[i].ID)
		if err != nil {
			return nil, true, err
		}
		boxes[i].Ports = append([]sbx.Port{}, ports...)
	}
	return boxes, true, nil
}

type textError string

func (e textError) Error() string { return string(e) }

func contextError(ctx context.Context, message string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return textError(message)
}

func (c *Catalog) commitSandboxes(boxes []sbx.Sandbox, installed bool, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sandbox.installed = installed
	c.sandbox.failed = err != nil && installed
	if err == nil {
		c.sandbox.boxes = cloneSandboxes(boxes)
		c.sandbox.hasGood = true
		c.updatedAt = time.Now()
	}
	c.health = c.healthLocked()
}

func (c *Catalog) healthLocked() Health {
	if c.generation == 0 && c.refreshing {
		return Refreshing
	}
	stale := false
	for _, source := range c.containers {
		if !source.failed {
			continue
		}
		if !source.hasGood {
			return Degraded
		}
		stale = true
	}
	if c.sandbox.failed {
		if !c.sandbox.hasGood {
			return Degraded
		}
		stale = true
	}
	if stale {
		return Stale
	}
	return Ready
}

func (c *Catalog) Snapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := Snapshot{
		Containers: make([]containers.Inventory, 0, len(c.containers)),
		Sandboxes:  cloneSandboxes(c.sandbox.boxes),
		Health:     c.health,
		Generation: c.generation,
		UpdatedAt:  c.updatedAt,
	}
	for _, source := range c.containers {
		if source.inventory.Engine != "" {
			result.Containers = append(result.Containers, cloneInventory(source.inventory))
		}
	}
	return result
}

func (c *Catalog) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		c.wg.Wait()
		return nil
	}
	c.closed = true
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.wg.Wait()
	return nil
}

func (c *Catalog) finishCancelledRefresh() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.refreshing {
		c.refreshing = false
		c.health = c.healthLocked()
		close(c.refreshDone)
	}
}

func cloneInventory(source containers.Inventory) containers.Inventory {
	result := source
	result.Containers = append([]containers.Container{}, source.Containers...)
	for i := range result.Containers {
		result.Containers[i].Ports = append([]containers.Port{}, source.Containers[i].Ports...)
	}
	return result
}

func cloneSandboxes(source []sbx.Sandbox) []sbx.Sandbox {
	result := append([]sbx.Sandbox{}, source...)
	for i := range result {
		result[i].Ports = append([]sbx.Port{}, source[i].Ports...)
	}
	return result
}
