// Package modulekit provides module-neutral lifecycle and HTTP helpers.
package modulekit

import (
	"context"
	"encoding/json"
	"github.com/zelport/zelport/internal/feature"
	"net/http"
	"sync"
	"time"
)

type Base struct {
	Host    feature.Host
	Desc    feature.Descriptor
	Summary func(feature.Route) feature.Summary
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func (b *Base) Descriptor() feature.Descriptor                  { return b.Desc }
func (b *Base) ValidateSettings(v json.RawMessage) error        { return feature.EmptySettings(v) }
func (b *Base) Start(ctx context.Context, h feature.Host) error { b.Host = h; return nil }
func (b *Base) Run(parent context.Context, refresh func(context.Context), interval time.Duration) {
	ctx, cancel := context.WithCancel(parent)
	b.cancel = cancel
	ch, stop := b.Host.Watch()
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer stop()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		refresh(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
			case <-ticker.C:
			}
			refresh(ctx)
		}
	}()
}
func (b *Base) Close() error {
	if b.cancel != nil {
		b.cancel()
	}
	b.wg.Wait()
	return nil
}
func (b *Base) Admin() http.Handler { return http.NotFoundHandler() }
func (b *Base) State() feature.State {
	s := b.Host.Snapshot()
	v := feature.State{Revision: s.Revision, Routes: []feature.RouteView{}, Data: json.RawMessage(`{}`)}
	for _, r := range s.Routes {
		summary := feature.Summary{State: "configured"}
		if b.Summary != nil {
			summary = b.Summary(r)
		}
		v.Routes = append(v.Routes, feature.View(r, summary))
	}
	return v
}
func JSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
