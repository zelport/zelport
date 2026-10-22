package app

import (
	"io"

	"github.com/zelport/zelport/internal/feature"
)

type tracked struct {
	c     io.Closer
	scope string
}

type active struct {
	lease  *feature.Lease
	scopes []string
}
