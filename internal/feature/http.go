package feature

import (
	"bufio"
	"encoding/json"
	"html/template"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/zelport/zelport/internal/diagnostic"
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

var statusPage = template.Must(template.New("status").Parse(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title><style>
:root{color-scheme:light dark;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#2b2533;background:#f7f5fa}*{box-sizing:border-box}body{margin:0}.page{max-width:760px;margin:12vh auto;padding:32px}h1{font-size:clamp(30px,4vw,46px);line-height:1.14;letter-spacing:-1.7px;margin:0 0 20px;font-weight:650}p{line-height:1.5;margin:0 0 24px}a,summary{color:#79589b}a:focus-visible,summary:focus-visible{outline:2px solid #79589b;outline-offset:3px}.actions{display:flex;align-items:center;gap:24px;flex-wrap:wrap}details{margin-top:28px;border-top:1px solid #e3dce9;padding-top:18px}summary{cursor:pointer;font-weight:600}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:#fefcff;border:1px solid #e3dce9;border-radius:8px;padding:16px;font:13px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace;user-select:text}@media(prefers-color-scheme:dark){:root{color:#eee7f4;background:#1d1923}a,summary{color:#c7a6e5}details,pre{border-color:#44384f}pre{background:#27212f}}
</style><main class="page"><h1>{{.Title}}</h1><p>{{.Message}}</p><div class="actions"><a href="">Try again</a></div>{{if .Diagnostic}}<details><summary>Technical details</summary><pre>{{.Diagnostic}}</pre></details>{{end}}</main></html>`))

func Status(w http.ResponseWriter, r *http.Request, code int, title, detail string) {
	status(w, r, code, title, detail, "")
}

// StatusDiagnostic keeps a raw backend or CLI error available without making it the main message.
func StatusDiagnostic(w http.ResponseWriter, r *http.Request, code int, title, message, raw string) {
	status(w, r, code, title, message, diagnostic.Clean(raw))
}

func status(w http.ResponseWriter, r *http.Request, code int, title, message, technical string) {
	w.Header().Set("Cache-Control", "no-store")
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		detail := message
		if technical != "" {
			detail = technical // Preserve the existing diagnostic field for API consumers.
		}
		json.NewEncoder(w).Encode(map[string]any{"error": title, "detail": detail, "status": code})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_ = statusPage.Execute(w, struct{ Title, Message, Diagnostic string }{title, message, technical})
}
