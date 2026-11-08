package ports_over_relay

import (
	"testing"

	"github.com/zelport/zelport-sdk-go/protocol"
	"github.com/zelport/zelport/internal/feature"
)

func TestRelayTraceHeaderMatchesProtocol(t *testing.T) {
	if feature.RelayTraceHeader != protocol.TraceHeader {
		t.Fatalf("feature.RelayTraceHeader = %q, protocol.TraceHeader = %q", feature.RelayTraceHeader, protocol.TraceHeader)
	}
}
