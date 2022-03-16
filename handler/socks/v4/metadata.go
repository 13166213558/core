package v4

import (
	"time"

	mdata "github.com/go-gost/core/metadata"
)

type metadata struct {
	readTimeout time.Duration
}

func (h *socks4Handler) parseMetadata(md mdata.Metadata) (err error) {
	const (
		readTimeout = "readTimeout"
	)

	h.md.readTimeout = mdata.GetDuration(md, readTimeout)
	return
}
