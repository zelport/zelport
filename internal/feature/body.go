package feature

import (
	"io"
	"net/http"
)

func DecodeBody(w http.ResponseWriter, r *http.Request, v any) error {
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if e != nil {
		return e
	}
	return Decode(b, v)
}
