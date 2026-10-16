package feature

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"sync"
)

func RequestLease(r *http.Request) *Lease { l, _ := r.Context().Value(leaseKey{}).(*Lease); return l }

type leasedConn struct {
	net.Conn
	once  sync.Once
	lease *Lease
}

func (c *leasedConn) Close() error {
	e := c.Conn.Close()
	c.once.Do(func() {
		c.lease.Close()
	})
	return e
}

type Writer struct {
	http.ResponseWriter
	Lease    *Lease
	Hijacked bool
}

func (w *Writer) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *Writer) Flush()                      { _ = http.NewResponseController(w.ResponseWriter).Flush() }
func (w *Writer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	c, b, e := http.NewResponseController(w.ResponseWriter).Hijack()
	if e != nil {
		return nil, nil, e
	}
	v := &leasedConn{Conn: c, lease: w.Lease}
	_, e = w.Lease.Track(c)
	if e != nil {
		return nil, nil, e
	}
	w.Hijacked = true
	return v, b, nil
}
func Status(w http.ResponseWriter, r *http.Request, code int, title, detail string) {
	w.Header().Set("Cache-Control", "no-store")
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]any{"error": title, "detail": detail, "status": code})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprintf(w, `<!doctype html><html lang="en"><meta charset="utf-8"><title>%s</title><style>body{font:16px system-ui;margin:10vh auto;max-width:650px;padding:24px;color:#34243e;background:#f8f5fc}a{color:#79569a}</style><h1>%s</h1><p>%s</p><p><a href="">Try again</a></p></html>`, template.HTMLEscapeString(title), template.HTMLEscapeString(title), template.HTMLEscapeString(detail))
}
