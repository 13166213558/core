package local

import (
	"time"

	mdata "github.com/go-gost/core/metadata"
)

type metadata struct {
	readTimeout time.Duration
}

func (h *forwardHandler) parseMetadata(md mdata.Metadata) (err error) {
	const (
		readTimeout = "readTimeout"
	)

	h.md.readTimeout = mdata.GetDuration(md, readTimeout)
	return
}
