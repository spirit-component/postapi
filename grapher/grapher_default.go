package grapher

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/go-spirit/spirit/worker/fbp"
	"github.com/go-spirit/spirit/worker/fbp/protocol"
	"github.com/gogap/config"
)

type defaultGrapher struct {
	graphs map[string]map[string][]*protocol.Port

	locker sync.Mutex
}

func init() {
	RegisterGrapher("default", newDefaultGrapher)
}

func newDefaultGrapher(conf config.Configuration) (grapher Grapher, err error) {
	provider := &defaultGrapher{
		graphs: make(map[string]map[string][]*protocol.Port),
	}

	err = provider.loadGraph(conf, false)

	if err != nil {
		return
	}

	grapher = provider

	return
}

func (p *defaultGrapher) Query(apiName string, header http.Header) (map[string]*protocol.Graph, bool) {
	g, exist := p.graphs[apiName]

	if !exist {
		return nil, false
	}

	graphs := make(map[string]*protocol.Graph)

	for k, v := range g {
		graphs[k] = &protocol.Graph{
			Seq:   1,
			Name:  k,
			Ports: v,
		}
	}

	// graphs[fbp.GraphNameOfNormal] = &protocol.Graph{
	// 	Seq:   1,
	// 	Name:  fbp.GraphNameOfNormal,
	// 	Ports: g.Normal,
	// }

	// graphs[fbp.GraphNameOfError] = &protocol.Graph{
	// 	Seq:   1,
	// 	Name:  fbp.GraphNameOfError,
	// 	Ports: g.Error,
	// }

	return graphs, true
}

func (p *defaultGrapher) WithFallback(conf config.Configuration) error {
	return p.loadGraph(conf, true)
}

func (p *defaultGrapher) loadGraph(conf config.Configuration, fallback bool) (err error) {

	if conf == nil {
		return
	}

	apiNames := conf.Keys()

	if len(apiNames) == 0 {
		return
	}

	p.locker.Lock()
	defer p.locker.Unlock()

	var graphs = make(map[string]map[string][]*protocol.Port)

	for _, apiKey := range apiNames {
		apiName := strings.Replace(apiKey, "-", ".", -1)

		apiConf := conf.GetConfig(apiKey)

		if apiConf == nil {
			err = fmt.Errorf("config of %s is nil", apiKey)
			return
		}

		apiName = apiConf.GetString("name", apiName)

		graphConf := apiConf.GetConfig("graph")

		if graphConf == nil {
			err = fmt.Errorf("graph of api %s is nil", apiKey)
			return
		}

		var apiGraphs = make(map[string][]*protocol.Port)

		for _, graphName := range graphConf.Keys() {
			portsConfig := graphConf.GetConfig(graphName)

			var ports []*protocol.Port
			ports, err = p.configToPorts(apiKey, portsConfig)

			if err != nil {
				return
			}

			for _, port := range ports {
				port.GraphName = graphName
			}

			apiGraphs[graphName] = ports
		}

		if _, exist := apiGraphs[fbp.GraphNameOfEntrypoint]; !exist {
			err = fmt.Errorf("api of '%s' has no entrypoint", apiKey)
			return
		}

		graphs[apiName] = apiGraphs
	}

	p.graphs = graphs

	return
}

func (p *defaultGrapher) configToPorts(apiName string, conf config.Configuration) (ports []*protocol.Port, err error) {

	if conf == nil {
		return
	}

	portNames := conf.Keys()

	var ret []*protocol.Port

	for _, name := range portNames {

		skip := conf.GetBoolean(name+".skip", false)
		if skip {
			continue
		}

		seq := conf.GetInt32(name + ".seq")
		if seq < 0 {
			err = fmt.Errorf("seq should greater than 0, api: %s, port %s", apiName, name)
			return
		}

		url := conf.GetString(name + ".url")
		if len(url) == 0 {
			err = fmt.Errorf("port url of %s in api %s is empty", name, apiName)
			return
		}

		metadataConf := conf.GetConfig(name + ".metadata")

		var metadata map[string]string

		if metadataConf != nil && len(metadataConf.Keys()) > 0 {

			metadata = make(map[string]string)

			for _, k := range metadataConf.Keys() {
				metadata[k] = metadataConf.GetString(k)
			}
		}

		ret = append(ret, &protocol.Port{
			Seq:      seq,
			Url:      url,
			Metadata: metadata,
		})
	}

	ports = ret

	return
}
