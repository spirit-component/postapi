package postapi

import (
	"github.com/go-spirit/spirit/worker/fbp/protocol"
	"github.com/gogap/config"
)

type GraphProvider interface {
	WithFallback(config.Configuration) error
	Query(apiName string) (map[string]*protocol.Graph, bool)
}
