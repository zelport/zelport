package feature

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// RemoteAdminContext is supplied by an authenticated built-in transport, never HTTP headers.
type RemoteAdminContext struct {
	ConsumerID string `json:"-"`
	ProviderID string `json:"device_id"`
	Hostname   string `json:"hostname"`
	// ConsumerHostname names the Zelport whose browser administers this one, so its UI can say where "back" is.
	ConsumerHostname string            `json:"consumer_hostname,omitempty"`
	Authority        string            `json:"authority"`
	ServiceLinks     map[string]string `json:"service_links"`
}
type AdministrationProvider interface {
	ServeAdministration(http.ResponseWriter, *http.Request) bool
}
type AdministrationHost interface {
	DispatchAdministration(http.ResponseWriter, *http.Request, RemoteAdminContext) error
}

func ReservedDomain(host string) bool {
	return host == "zelport.localhost" || strings.HasSuffix(host, ".zelport.localhost")
}

// ErrRemoteFileAccess rejects host file paths and contents in remote administration.
var ErrRemoteFileAccess = errors.New("Remote administration cannot change files on this computer. Allow remote file access in Settings on this computer.")

type remoteFileAccessKey struct{}

// WithRemoteAdministration marks a request received through authenticated remote
// administration and records whether this computer's policy allows file access.
func WithRemoteAdministration(ctx context.Context, fileAccess bool) context.Context {
	return context.WithValue(ctx, remoteFileAccessKey{}, fileAccess)
}

// HostFileAccess reports whether a request may read or change host file paths and
// contents. Local administration always may; remote administration only when allowed.
func HostFileAccess(r *http.Request) bool {
	allowed, remote := r.Context().Value(remoteFileAccessKey{}).(bool)
	return !remote || allowed
}
