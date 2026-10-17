// Package feature defines the standard-library-only extension contract.
package feature

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"time"
)

var ErrConflict = errors.New("configuration changed or domain already in use; refresh and retry")
var ErrNotFound = errors.New("route or module not found")
var ErrClosed = errors.New("service is no longer available")

type Route struct {
	ID       string          `json:"id"`
	Module   string          `json:"module"`
	Domain   string          `json:"domain"`
	Settings json.RawMessage `json:"settings"`
}

const (
	Stable       = "stable"
	Experimental = "experimental"
)

type Descriptor struct {
	Maturity     string        `json:"maturity"`
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	TabID        string        `json:"tab_id"`
	UIPlacement  string        `json:"ui_placement,omitempty"` // Empty means a standalone tab; "settings" mounts a Settings section.
	Order        int           `json:"order"`
	Exportable   bool          `json:"exportable"`
	PollInterval time.Duration `json:"poll_interval"`
	Assets       fs.FS         `json:"-"`
}
type Summary struct {
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
	Target string `json:"target,omitempty"`
}
type RouteView struct {
	ID      string          `json:"id"`
	Module  string          `json:"module"`
	Domain  string          `json:"domain"`
	Summary Summary         `json:"summary"`
	Editor  json.RawMessage `json:"editor"`
}
type State struct {
	Revision uint64          `json:"revision"`
	Routes   []RouteView     `json:"routes"`
	Data     json.RawMessage `json:"data"`
}
type Snapshot struct {
	EditRevision uint64 // Latest user-visible mutation; internal metadata does not advance it.
	Revision     uint64
	Routes       []Route
	Settings     json.RawMessage
}
type Scope struct {
	Name       string
	Generation uint64
}
type CommitOptions struct {
	// StrictRevision rejects any intervening write, including metadata updates.
	StrictRevision bool
	ResetScopes    []string
	// MetadataOnly persists counters and watermarks using strict CAS. It must not
	// change route definitions, authorization, credentials, or effective settings.
	MetadataOnly bool
}
type CommitResult struct{ Revision uint64 }
type Draft struct {
	Routes   []Route
	Settings json.RawMessage
}
type Host interface {
	Snapshot() Snapshot
	OwnServices() []Route
	ListenerAuthority(string) string
	Watch() (<-chan struct{}, func())
	Commit(context.Context, uint64, CommitOptions, func(*Draft) error) (CommitResult, error)
	Scope(string) Scope
	Acquire(context.Context, string, uint64, ...Scope) (*Lease, error)
	Track(uint64, Scope, io.Closer) (func(), error)
	Dispatch(http.ResponseWriter, *http.Request, *Lease)
	AddMiddleware(func(http.Handler) http.Handler) func()
}
type Module interface {
	Descriptor() Descriptor
	ValidateRoute(Route) error
	ValidateSettings(json.RawMessage) error
	Start(context.Context, Host) error
	Close() error
	State() State
	ServeRoute(http.ResponseWriter, *http.Request, Route)
	Admin() http.Handler
}

func View(r Route, s Summary) RouteView {
	return RouteView{ID: r.ID, Module: r.Module, Domain: r.Domain, Summary: s, Editor: append(json.RawMessage(nil), r.Settings...)}
}

// Settings lets small modules accept only the empty settings document.
func EmptySettings(b json.RawMessage) error { var v struct{}; return Decode(b, &v) }

// AvailabilityGuard supplies a pure, configuration-based reason to keep a module UI
// available. Implementations must not access the host or acquire runtime locks.
type AvailabilityGuard interface {
	UnavailableReason(json.RawMessage) string
}

// ConnectionSummary is a public, secret-free desktop connection indicator.
type ConnectionSummary struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	State string `json:"state"`
}

// ConnectionSummaryProvider is optional; implementations return only display state.
type ConnectionSummaryProvider interface{ ConnectionSummaries() []ConnectionSummary }
