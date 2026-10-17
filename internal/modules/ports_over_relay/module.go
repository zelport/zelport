package ports_over_relay

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"github.com/zelport/zelport-sdk-go/client"
	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
	"github.com/zelport/zelport/internal/modulekit"
	"io/fs"
	"sync"
	"time"
)

//go:embed assets/*
var assets embed.FS

type Module struct {
	modulekit.Base
	store            store
	mu               sync.Mutex
	client           *client.Client
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	removeMiddleware func()
	error            string
}

func New() feature.Module {
	files, _ := fs.Sub(assets, "assets")
	m := &Module{}
	m.Desc = feature.Descriptor{Maturity: feature.Experimental, ID: "ports_over_relay", Name: "Ports over Relay", TabID: "cloud", Order: 60, PollInterval: 15 * time.Second, Assets: files}
	return m
}
func (m *Module) Start(parent context.Context, h feature.Host) error {
	m.Base.Start(parent, h)
	m.store = store{h}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.removeMiddleware = h.AddMiddleware(m.middleware)
	changed, unsubscribe := h.Watch()
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer unsubscribe()
		var oldConnection, oldCatalog string
		for {
			v, rev := m.store.snapshot()
			connection, _ := json.Marshal([]any{v.Enabled, v.RelayURL, v.AccessToken, v.GroupToken, v.DeviceID, v.PrivateKey})
			catalog, _ := json.Marshal([]any{v.Publish, h.OwnServices()})
			if string(connection) != oldConnection {
				if m.replaceClient(ctx, v, rev) {
					oldConnection = string(connection)
				}
			}
			if string(catalog) != oldCatalog {
				m.mu.Lock()
				c := m.client
				m.mu.Unlock()
				if c != nil {
					c.Notify()
				}
				oldCatalog = string(catalog)
			}
			select {
			case <-ctx.Done():
				return
			case _, ok := <-changed:
				if !ok {
					return
				}
			}
		}
	}()
	return nil
}
func (m *Module) replaceClient(ctx context.Context, s Settings, revision uint64) bool {
	m.mu.Lock()
	old := m.client
	m.client = nil
	m.mu.Unlock()
	if old != nil {
		old.Close()
	}
	if !s.Enabled {
		return true
	}
	o, e := m.clientOptions(s)
	if e != nil {
		return false
	}
	c, e := client.New(o)
	if e != nil {
		return false
	}
	release, e := m.Host.Track(revision, m.Host.Scope("client"), c)
	if e != nil {
		c.Close()
		return false
	}
	m.mu.Lock()
	m.client = c
	m.mu.Unlock()
	m.wg.Add(1)
	go func() { defer m.wg.Done(); defer release(); c.Run(ctx) }()
	return true
}
func (m *Module) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Lock()
	c := m.client
	m.mu.Unlock()
	if c != nil {
		c.Close()
	}
	m.wg.Wait()
	if m.removeMiddleware != nil {
		m.removeMiddleware()
	}
	return nil
}
func (m *Module) State() feature.State {
	s := m.Base.State()
	v, _ := m.store.snapshot()
	status := client.Status{Records: []protocol.Record{}}
	m.mu.Lock()
	c := m.client
	m.mu.Unlock()
	if c != nil {
		status = c.Snapshot()
	}
	s.Data, _ = json.Marshal(map[string]any{"enabled": v.Enabled, "publish": v.Publish, "relay_url": v.RelayURL, "device_id": v.DeviceID, "has_identity": v.DeviceID != "", "has_access_token": v.AccessToken != "", "has_group_token": v.GroupToken != "", "status": status, "administration_peers": m.adminPeers(v, status)})
	for i := range s.Routes {
		var ref ImportSettings
		feature.Decode(s.Routes[i].Editor, &ref)
		s.Routes[i].Summary = feature.Summary{State: "unavailable", Detail: "Relay service is unavailable. Import is preserved.", Target: ref.DeviceID}
		if m.validateOffer(ref) == nil {
			s.Routes[i].Summary.State = "ready"
			s.Routes[i].Summary.Detail = ""
		}
	}
	return s
}
func (m *Module) validateOffer(s ImportSettings) error {
	m.mu.Lock()
	c := m.client
	m.mu.Unlock()
	if c == nil {
		return errors.New("relay catalog is unavailable")
	}
	status := c.Snapshot()
	if !status.Connected {
		return errors.New("relay catalog is unavailable")
	}
	for _, r := range status.Records {
		if r.DeviceID == s.DeviceID && r.Group == s.Group && r.PublicKey == s.ProviderKey && r.ExpiresAt > time.Now().Unix() {
			for _, service := range r.Services {
				if service.ID == s.ServiceID && service.Kind == protocol.Application {
					_, e := r.Authority(s.ServiceID)
					return e
				}
			}
			return errors.New("built-in administration cannot be imported")
		}
	}
	return errors.New("authenticated relay service is unavailable")
}

// UnavailableReason reads desired connection state, not transient connectivity.
func (m *Module) UnavailableReason(raw json.RawMessage) string {
	var s Settings
	if err := feature.Decode(raw, &s); err != nil {
		return "Cannot read relay connection settings."
	}
	if s.Enabled {
		return "Turn off Connect to relay before disabling this module."
	}
	return ""
}

func (m *Module) ConnectionSummaries() []feature.ConnectionSummary {
	settings, _ := m.store.snapshot()
	state := "disconnected"
	if settings.Enabled {
		state = "connecting"
		m.mu.Lock()
		c := m.client
		failure := m.error
		m.mu.Unlock()
		if failure != "" {
			state = "error"
		}
		if c != nil {
			status := c.Snapshot()
			if status.Connected {
				state = "connected"
			} else if status.Error != "" {
				state = "error"
			}
		}
	}
	return []feature.ConnectionSummary{{ID: "ports_over_relay", Label: "Ports over Relay", State: state}}
}
