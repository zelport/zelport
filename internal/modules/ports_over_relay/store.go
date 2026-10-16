package ports_over_relay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/zelport/zelport/internal/feature"
)

type store struct{ host feature.Host }

func (s store) snapshot() (Settings, uint64) {
	v := s.host.Snapshot()
	var settings Settings
	feature.Decode(v.Settings, &settings)
	return settings, v.Revision
}

// update retries pure internal changes only. User mutations use an explicit revision.
func (s store) update(ctx context.Context, options feature.CommitOptions, mutate func(*Settings) error) error {
	return s.updateClassified(ctx, options, false, mutate)
}

func (s store) updateClassified(ctx context.Context, options feature.CommitOptions, classifyPeers bool, mutate func(*Settings) error) error {
	options.StrictRevision = true
	for {
		if e := ctx.Err(); e != nil {
			return e
		}
		v, rev := s.snapshot()
		original, _ := json.Marshal(v)
		before, _ := json.Marshal(v.Peers)
		if e := mutate(&v); e != nil {
			return e
		}
		updated, _ := json.Marshal(v)
		if bytes.Equal(original, updated) {
			return nil
		}
		opt := options
		if classifyPeers {
			after, _ := json.Marshal(v.Peers)
			opt.MetadataOnly = bytes.Equal(before, after)
		}
		_, e := s.host.Commit(ctx, rev, opt, func(d *feature.Draft) error { b, e := json.Marshal(v); d.Settings = b; return e })
		if !errors.Is(e, feature.ErrConflict) {
			return e
		}
	}
}
