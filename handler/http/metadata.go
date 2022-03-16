package http

import (
	"net/http"
	"strings"

	mdata "github.com/go-gost/core/metadata"
)

type metadata struct {
	probeResistance *probeResistance
	sni             bool
	enableUDP       bool
	header          http.Header
}

func (h *httpHandler) parseMetadata(md mdata.Metadata) error {
	const (
		header         = "header"
		probeResistKey = "probeResistance"
		knock          = "knock"
		sni            = "sni"
		enableUDP      = "udp"
	)

	if m := mdata.GetStringMapString(md, header); len(m) > 0 {
		hd := http.Header{}
		for k, v := range m {
			hd.Add(k, v)
		}
		h.md.header = hd
	}

	if v := mdata.GetString(md, probeResistKey); v != "" {
		if ss := strings.SplitN(v, ":", 2); len(ss) == 2 {
			h.md.probeResistance = &probeResistance{
				Type:  ss[0],
				Value: ss[1],
				Knock: mdata.GetString(md, knock),
			}
		}
	}
	h.md.sni = mdata.GetBool(md, sni)
	h.md.enableUDP = mdata.GetBool(md, enableUDP)

	return nil
}

type probeResistance struct {
	Type  string
	Value string
	Knock string
}
