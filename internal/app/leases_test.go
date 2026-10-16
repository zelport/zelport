package app

import (
	"context"
	"encoding/json"
	"github.com/zelport/zelport/internal/feature"
	"testing"
	"time"
)

type reviewBlockedCloser struct{ entered, unblock chan struct{} }

func (c reviewBlockedCloser) Close() error { close(c.entered); <-c.unblock; return nil }
func TestCommitWaitsForInflightLeaseCleanup(t *testing.T) {
	a := &testModule{id: "x"}
	m := setup(t, a)
	r, e := m.Save(context.Background(), 0, feature.Route{Module: "x", Domain: "one.localhost", Settings: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	l, e := a.host.Acquire(context.Background(), r.ID, 1)
	if e != nil {
		t.Fatal(e)
	}
	entered, unblock := make(chan struct{}), make(chan struct{})
	defer close(unblock)
	l.Track(reviewBlockedCloser{entered, unblock})
	go l.Close()
	<-entered
	r.Domain = "two.localhost"
	committed := make(chan error, 1)
	go func() { _, e := m.Save(context.Background(), 1, r); committed <- e }()
	select {
	case e := <-committed:
		if e != nil {
			t.Fatal(e)
		}
		t.Fatal("route mutation acknowledged while lease backend cleanup is still blocked")
	case <-time.After(100 * time.Millisecond):
	}
}
