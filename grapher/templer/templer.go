package templer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"text/template"

	"github.com/go-spirit/spirit/worker/fbp/protocol"
	"github.com/gogap/config"
	"github.com/sirupsen/logrus"
	"github.com/spirit-component/postapi/grapher"
)

type TemplerGrapher struct {
	conf        config.Configuration
	apiTempl    map[string]*template.Template
	defaultTmpl *template.Template
}

func init() {
	grapher.RegisterGrapher("templer", NewTemplerGrapher)
}

func NewTemplerGrapher(conf config.Configuration) (g grapher.Grapher, err error) {
	templer := &TemplerGrapher{
		conf:     conf,
		apiTempl: make(map[string]*template.Template),
	}

	if conf == nil {
		g = templer
		return
	}

	conf.Keys()

	for _, k := range conf.Keys() {
		var tmpl *template.Template

		tmplFile := conf.GetString(fmt.Sprintf("%s.template", k))

		tmpl, err = templer.readTemplate(tmplFile)

		if err != nil {
			return
		}

		if k == "default" {
			templer.defaultTmpl = tmpl
			continue
		}

		apiName := conf.GetString("name", strings.Replace(k, "-", ".", -1))
		templer.apiTempl[apiName] = tmpl
	}

	g = templer

	return
}

func (p *TemplerGrapher) WithFallback(conf config.Configuration) error {
	return nil
}

func (p *TemplerGrapher) Query(apiName string, header http.Header) (map[string]*protocol.Graph, bool) {

	tmpl, exist := p.apiTempl[apiName]
	if !exist {
		tmpl = p.defaultTmpl
	}

	if tmpl == nil {
		return nil, false
	}

	buf := bytes.NewBuffer(nil)
	err := tmpl.Execute(buf, map[string]interface{}{"api": apiName, "header": header})
	if err != nil {
		logrus.WithField("components", "postapi").WithField("grapher", "templer").WithError(err).Errorln("query api graph")
		return nil, false
	}

	decoder := json.NewDecoder(buf)
	decoder.UseNumber()

	ret := map[string]*protocol.Graph{}

	err = decoder.Decode(&ret)
	if err != nil {
		logrus.WithField("components", "postapi").WithField("grapher", "templer").WithError(err).Errorln("query api graph")
		return nil, false
	}

	for k, _ := range ret {
		ret[k].Name = k
		ret[k].Seq = 1
	}

	return ret, true
}

func (p *TemplerGrapher) readTemplate(filename string) (tmpl *template.Template, err error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	tmpl, err = template.New(filename).Parse(string(data))
	return
}
