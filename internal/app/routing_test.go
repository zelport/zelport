package app

import (
	"context"
	"encoding/json"
	"github.com/zelport/zelport/internal/feature"
	"net/http/httptest"
	"testing"
)

func TestDomainLookupMustMatchAcquiredRoute(t *testing.T) {
	mod := &testModule{id: "x"}
	m, e := New("", 8080, mod)
	if e != nil {
		t.Fatal(e)
	}
	m.Start(context.Background())
	defer m.Close()
	r, e := m.Save(context.Background(), 0, feature.Route{Module: "x", Domain: "one.localhost", Settings: json.RawMessage(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	stop, done := make(chan struct{}), make(chan struct{})
	defer func() { close(stop); <-done }()
	go func() {
		defer close(done)
		rev := uint64(1)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if rev%2 == 1 {
				r.Domain = "two.localhost"
			} else {
				r.Domain = "one.localhost"
			}
			if _, err := m.Save(context.Background(), rev, r); err != nil {
				return
			}
			rev++
		}
	}()
	for i := 0; i < 20000; i++ {
		w := httptest.NewRecorder()
		m.ServeHTTP(w, httptest.NewRequest("GET", "http://one.localhost:8080/", nil))
		if w.Code == 200 && w.Body.String() != "one.localhost" {
			t.Fatalf("request to one.localhost dispatched renamed route %q", w.Body.String())
		}
	}
}
