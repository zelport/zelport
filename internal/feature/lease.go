package feature

import (
	"context"
	"io"
	"sync"
)

type Lease struct {
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	route      Route
	generation uint64
	items      map[uint64]io.Closer
	seq        uint64
	closed     bool
	done       func()
	stop       func() bool
	completed  chan struct{}
}

func NewLease(ctx context.Context, r Route, g uint64, done func()) *Lease {
	c, cancel := context.WithCancel(ctx)
	l := &Lease{ctx: c, cancel: cancel, route: r, generation: g, items: map[uint64]io.Closer{}, done: done, completed: make(chan struct{})}
	l.stop = context.AfterFunc(c, func() { l.Close() })
	return l
}
func (l *Lease) Context() context.Context { return l.ctx }
func (l *Lease) Generation() uint64       { return l.generation }
func (l *Lease) Route() Route {
	r := l.route
	r.Settings = append([]byte(nil), r.Settings...)
	return r
}
func (l *Lease) Track(c io.Closer) (func(), error) {
	l.mu.Lock()
	if l.closed || l.ctx.Err() != nil {
		l.mu.Unlock()
		c.Close()
		return func() {}, ErrClosed
	}
	l.seq++
	n := l.seq
	l.items[n] = c
	l.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { l.mu.Lock(); delete(l.items, n); l.mu.Unlock() }) }, nil
}
func (l *Lease) Close() error {
	l.mu.Lock()
	if l.closed {
		completed := l.completed
		l.mu.Unlock()
		<-completed
		return nil
	}
	l.closed = true
	l.cancel()
	items := l.items
	l.items = nil
	done := l.done
	l.mu.Unlock()
	for _, v := range items {
		v.Close()
	}
	if done != nil {
		done()
	}
	close(l.completed)
	return nil
}

type leaseKey struct{}

func WithLease(rctx context.Context, l *Lease) context.Context {
	return context.WithValue(rctx, leaseKey{}, l)
}
