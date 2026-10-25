package configfile

import (
	"github.com/zelport/zelport/internal/feature"
	"io"
	"net/http"
	"unicode/utf8"
)

type Update struct {
	Revision uint64 `json:"revision"`
	Version  string `json:"version"`
	Content  string `json:"content"`
}

// JSON may expand each UTF-8 byte to six bytes. Bound both the wire request and
// decoded content instead of accidentally limiting PEM/text to less than 1 MiB.
func DecodeUpdate(w http.ResponseWriter, r *http.Request) (Update, error) {
	var update Update
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 6*MaxSize+4096))
	if err == nil {
		if !utf8.Valid(data) {
			return update, ErrInvalid
		}
		err = feature.Decode(data, &update)
	}
	if err != nil {
		return update, ErrInvalid
	}
	return update, nil
}
