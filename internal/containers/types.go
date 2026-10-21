// Package containers discovers existing published ports without changing engines.
package containers

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

type Port struct {
	HostIP        string `json:"host_ip"`
	PublishedIP   string `json:"published_ip,omitempty"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

func (p Port) Binding() (netip.Addr, bool) {
	value := p.PublishedIP
	if value == "" {
		value = p.HostIP
	}
	ip, err := netip.ParseAddr(value)
	return ip, err == nil
}

func (p Port) Usable() bool {
	ip := net.ParseIP(p.HostIP)
	return ip != nil && ip.IsLoopback() && p.Protocol == "tcp" && p.HostPort > 0 && p.HostPort <= 65535 && p.ContainerPort > 0 && p.ContainerPort <= 65535
}
func (p Port) Address() string { return net.JoinHostPort(p.HostIP, strconv.Itoa(p.HostPort)) }

type Container struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Ports  []Port `json:"ports"`
}
type Inventory struct {
	CLIStatus     string      `json:"cli_status"`
	CLIPath       string      `json:"cli_path,omitempty"`
	RuntimeStatus string      `json:"runtime_status"`
	Endpoint      string      `json:"endpoint,omitempty"`
	Namespace     string      `json:"namespace,omitempty"`
	Engine        string      `json:"engine"`
	Connection    string      `json:"connection"`
	Label         string      `json:"label"`
	Containers    []Container `json:"containers"`
	Stale         bool        `json:"stale"`
	Error         string      `json:"error,omitempty"`
	UpdatedAt     time.Time   `json:"updated_at"`
}
type Reference struct {
	Engine     string `json:"engine"`
	Connection string `json:"connection"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Port       int    `json:"port"`
	Selected   Port   `json:"selected"`
}
type Backend interface {
	Scan(context.Context) Inventory
}

type Catalog struct {
	mu       sync.RWMutex
	sources  []Inventory
	backends []Backend
	locks    []sync.Mutex
}

func NewCatalog(backends ...Backend) *Catalog {
	return &Catalog{backends: backends, sources: make([]Inventory, len(backends)), locks: make([]sync.Mutex, len(backends))}
}
func (c *Catalog) Snapshot() []Inventory {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Inventory, 0, len(c.sources))
	for _, s := range c.sources {
		if s.Engine == "" {
			continue
		}
		s.Containers = append([]Container{}, s.Containers...)
		for i := range s.Containers {
			s.Containers[i].Ports = append([]Port{}, s.Containers[i].Ports...)
		}
		result = append(result, s)
	}
	return result
}

// Each source commits independently, even while another engine is timing out.
func (c *Catalog) Refresh(ctx context.Context) {
	var wg sync.WaitGroup
	for i, b := range c.backends {
		wg.Add(1)
		go func(i int, b Backend) {
			defer wg.Done()
			c.locks[i].Lock()
			defer c.locks[i].Unlock()
			next := b.Scan(ctx)
			c.mu.Lock()
			defer c.mu.Unlock()
			old := c.sources[i]
			if next.Stale && (next.Connection == "" || next.Connection == old.Connection) {
				next.Containers = old.Containers
				next.UpdatedAt = old.UpdatedAt
				if next.Connection == "" {
					next.Connection = old.Connection
					next.Label = old.Label
				}
			}
			c.sources[i] = next
		}(i, b)
	}
	wg.Wait()
}
