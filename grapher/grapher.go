package grapher

import (
	"errors"
	"fmt"

	"github.com/go-spirit/spirit/worker/fbp/protocol"
	"github.com/gogap/config"
)

type postAPIPorts struct {
	Error  []*protocol.Port
	Normal []*protocol.Port
}

type Grapher interface {
	WithFallback(config.Configuration) error
	Query(apiName string) (map[string]*protocol.Graph, bool)
}

type NewGrapherFunc func(config.Configuration) (Grapher, error)

var (
	graphers = make(map[string]NewGrapherFunc)
)

func RegisterGrapher(name string, fn NewGrapherFunc) (err error) {
	if len(name) == 0 {
		err = errors.New("grapher name is empty")
		return
	}

	if fn == nil {
		err = fmt.Errorf("new grapher func is nil, %s", name)
		return
	}

	_, exist := graphers[name]

	if exist {
		err = fmt.Errorf("grapher %s already registered", name)
		return
	}

	graphers[name] = fn

	return
}

func NewGrapher(name string, conf config.Configuration) (g Grapher, err error) {
	if len(name) == 0 {
		err = errors.New("grapher name is empty")
		return
	}

	fn, exist := graphers[name]

	if !exist {
		err = fmt.Errorf("grapher %s not registered", name)
		return
	}

	g, err = fn(conf)

	return
}
