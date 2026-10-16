package feature

import (
	"net/http"
	"strings"
)

// RemoteAdminContext is supplied by an authenticated built-in transport, never HTTP headers.
type RemoteAdminContext struct {
	ConsumerID   string            `json:"-"`
	ProviderID   string            `json:"device_id"`
	Hostname     string            `json:"hostname"`
	Authority    string            `json:"authority"`
	ServiceLinks map[string]string `json:"service_links"`
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
