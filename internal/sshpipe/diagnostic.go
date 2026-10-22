package sshpipe

import (
	"io"
	"strings"
	"sync"

	"github.com/zelport/zelport/internal/diagnostic"
)

type diagnosticBuffer struct {
	mu sync.Mutex
	b  []byte
}

func (d *diagnosticBuffer) Write(b []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(b)
	d.b = append(d.b, b...)
	if len(d.b) > 4096 {
		d.b = append([]byte(nil), d.b[len(d.b)-4096:]...)
	}
	return n, nil
}

func (d *diagnosticBuffer) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return diagnostic.Clean(string(d.b))
}

// A jump host can report a forwarding failure to the destination SSH port.
// A subsequent key-exchange failure belongs to SSH connectivity, not HTTP.
func backendFailure(message string) bool {
	return strings.Contains(message, "open failed: connect failed:") &&
		!strings.Contains(message, "kex_exchange_identification") &&
		!strings.Contains(message, "ssh_exchange_identification") &&
		!strings.Contains(message, "Connection closed by UNKNOWN") &&
		!strings.Contains(message, "banner exchange")
}

var _ io.Writer = (*diagnosticBuffer)(nil)
