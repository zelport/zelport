package app

import (
	"errors"
	"github.com/zelport/zelport/internal/feature"
	"net/http"
)

// SetAdministrationHandler wires the trusted internal bridge before modules start.
func (m *Manager) SetAdministrationHandler(handler func(http.ResponseWriter, *http.Request, feature.RemoteAdminContext)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.administration = handler
}
func (h *host) DispatchAdministration(w http.ResponseWriter, r *http.Request, ctx feature.RemoteAdminContext) error {
	h.m.mu.RLock()
	handler := h.m.administration
	closed := h.m.closed
	h.m.mu.RUnlock()
	if handler == nil || closed {
		return errors.New("administration unavailable")
	}
	if _, ok := h.m.modules[h.id].(feature.AdministrationProvider); !ok {
		return errors.New("module does not provide administration")
	}
	handler(w, r, ctx)
	return nil
}
