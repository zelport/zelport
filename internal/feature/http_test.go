package feature

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type pipeWriter struct {
	http.ResponseWriter
	c net.Conn
}

func (w pipeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.c, bufio.NewReadWriter(bufio.NewReader(w.c), bufio.NewWriter(w.c)), nil
}
func TestHijackCloseRace(t *testing.T) {
	for i := 0; i < 10000; i++ {
		a, b := net.Pipe()
		l := NewLease(context.Background(), Route{}, 1, nil)
		w := Writer{ResponseWriter: pipeWriter{httptest.NewRecorder(), a}, Lease: l}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); l.Close() }()
		c, _, _ := w.Hijack()
		if c != nil {
			c.Close()
		}
		wg.Wait()
		b.Close()
	}
}
