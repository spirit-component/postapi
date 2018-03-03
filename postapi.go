package postapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gogap/config"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"

	"github.com/go-spirit/spirit/component"
	"github.com/go-spirit/spirit/doc"
	"github.com/go-spirit/spirit/mail"
	"github.com/go-spirit/spirit/message"
	"github.com/go-spirit/spirit/worker"
	"github.com/go-spirit/spirit/worker/fbp"
	"github.com/go-spirit/spirit/worker/fbp/protocol"
	"github.com/spirit-component/postapi/grapher"
)

const (
	XApi        = "X-Api"
	XApiBatch   = "X-Api-Batch"
	XApiTimeout = "X-Api-Timeout"
)

type ctxHttpComponentKey struct{}

type httpCacheItem struct {
	c    *gin.Context
	ctx  context.Context
	done chan *PostAPIResponse
}

type PostAPIResponse struct {
	API            string          `json:"-"`
	Code           int64           `json:"code"`
	ErrorId        string          `json:"error_id,omitempty"`
	ErrorNamespace string          `json:"error_namespace,omitempty"`
	Message        string          `json:"message,omitempty"`
	Result         json.RawMessage `json:"result,omitempty"`
}

type PostAPI struct {
	opts  component.Options
	alias string

	grapher grapher.Grapher

	srv *http.Server
}

func init() {
	component.RegisterComponent("post-api", NewPostAPI)
	doc.RegisterDocumenter("post-api", &PostAPI{})
}

func NewPostAPI(alias string, opts ...component.Option) (srv component.Component, err error) {
	s := &PostAPI{
		alias: alias,
	}

	err = s.init(opts...)
	if err != nil {
		return
	}

	srv = s
	return
}

func (p *PostAPI) init(opts ...component.Option) (err error) {

	for _, o := range opts {
		o(&p.opts)
	}

	grapherDriver := p.opts.Config.GetString("grapher.driver", "default")

	if len(grapherDriver) == 0 {
		err = errors.New("post api grapher driver is empty")
		return
	}

	grapherConf := p.opts.Config.GetConfig(fmt.Sprintf("grapher.%s", grapherDriver))

	apiGrapher, err := grapher.NewGrapher(grapherDriver, grapherConf)

	if err != nil {
		return
	}

	p.grapher = apiGrapher

	debug := p.opts.Config.GetBoolean("debug", false)
	if !debug {
		gin.SetMode("release")
	}

	urlPath := p.opts.Config.GetString("path", "/")

	router := gin.New()
	router.Use(gin.Recovery())
	router.POST(urlPath, p.serve)

	httpConf := p.opts.Config.GetConfig("http")

	if httpConf == nil {
		httpConf = config.NewConfig()
	}

	p.loadCORS(router, httpConf.GetConfig("cors"))
	p.loadPprof(router, httpConf.GetConfig("pprof"))

	address := httpConf.GetString("address", ":8080")

	p.srv = &http.Server{
		Addr:    address,
		Handler: router,
	}

	return
}

func (p *PostAPI) call(apiName string, body []byte, timeout time.Duration, c *gin.Context) (resp *PostAPIResponse) {

	graphs, exist := p.grapher.Query(apiName, c.Request.Header)

	if !exist {
		resp = &PostAPIResponse{
			API:            apiName,
			ErrorNamespace: "POST-API",
			Code:           http.StatusNotFound,
			Message:        "Api Not Found",
		}
		return
	}

	id := uuid.New()
	payload := &protocol.Payload{
		Id:           id,
		Timestamp:    time.Now().UnixNano(),
		CurrentGraph: fbp.GraphNameOfEntrypoint,
		Graphs:       graphs,
		Message: &protocol.Message{
			Id:     id,
			Header: map[string]string{"content-type": "application/json"},
			Body:   body,
		},
	}

	graph, exist := payload.GetGraph(fbp.GraphNameOfEntrypoint)
	if !exist {
		resp = &PostAPIResponse{
			API:            apiName,
			ErrorNamespace: "POST-API",
			Code:           http.StatusInternalServerError,
			Message:        "Internal Error (Api graph not found)",
		}
		return
	}

	port, err := graph.CurrentPort()

	if err != nil {
		resp = &PostAPIResponse{
			API:            apiName,
			ErrorNamespace: "POST-API",
			Code:           http.StatusInternalServerError,
			Message:        "Internal Error (bad graph)",
		}
		return
	}

	session := mail.NewSession()

	session.WithPayload(payload)
	session.WithFromTo("", port.GetUrl())

	fbp.SessionWithPort(session, graph.GetName(), port.GetUrl(), port.GetMetadata())

	var ctx context.Context
	var cancel context.CancelFunc
	var doneChan chan *PostAPIResponse

	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()

		doneChan = make(chan *PostAPIResponse)
		defer close(doneChan)

		// storage response object to cache
		p.opts.Cache.Set(p.cacheKey(id), &httpCacheItem{c, ctx, doneChan})
	} else {
		p.opts.Cache.Set(p.cacheKey(id), (*httpCacheItem)(nil))
	}

	err = p.opts.Postman.Post(
		message.NewUserMessage(session),
	)

	if err != nil {

		logrus.WithField("component", "post-api").
			WithField("alias", p.alias).
			WithField("from", session.From()).
			WithField("to", session.To()).
			WithField("seq", graph.GetSeq()).
			WithError(err).Errorln("post user message failure")

		resp = &PostAPIResponse{
			API:            apiName,
			ErrorNamespace: "POST-API",
			Code:           http.StatusInternalServerError,
			Message:        "Internal Error - post failure",
		}

		return
	}

	if timeout <= 0 {
		resp = &PostAPIResponse{}
		resp.API = apiName
		return
	}

	for {
		select {
		case r := <-doneChan:
			{
				resp = r
				resp.API = apiName
				return
			}
		case <-ctx.Done():
			{
				resp = &PostAPIResponse{
					API:            apiName,
					ErrorNamespace: "POST-API",
					Code:           http.StatusRequestTimeout,
					Message:        "Request Timeout",
				}

				return
			}
		}
	}
}

type batchApiCallReq map[string]json.RawMessage

func (p *PostAPI) serve(c *gin.Context) {

	var err error

	strIsBatchCall := c.GetHeader(XApiBatch)

	isBatchCall := false

	if len(strIsBatchCall) > 0 {

		isBatchCall, err = strconv.ParseBool(strIsBatchCall)

		if err != nil {
			c.JSON(
				http.StatusOK,
				PostAPIResponse{
					ErrorNamespace: "POST-API",
					Code:           http.StatusBadRequest,
					Message:        "Bad request - batch call header error",
				},
			)

			return
		}

	}

	if isBatchCall {
		err = p.serveBatchCall(c)
	} else {
		err = p.serveSingleCall(c)
	}

	if err != nil {
		c.JSON(
			http.StatusOK,
			PostAPIResponse{
				ErrorNamespace: "POST-API",
				Code:           http.StatusBadRequest,
				Message:        "Bad request",
			},
		)

		logrus.WithField("component", "post-api").
			WithField("alias", p.alias).
			WithField("is-batch", isBatchCall).
			WithField("X-Api", c.GetHeader(XApi)).
			WithError(err).Errorln("serve request failure")

		return
	}
}

func (p *PostAPI) serveBatchCall(c *gin.Context) (err error) {

	batchReq := batchApiCallReq{}

	err = c.ShouldBindJSON(&batchReq)
	if err != nil {
		return
	}

	preperReqs := make(map[string][]byte)

	for apiName, jsonData := range batchReq {
		_, exist := p.grapher.Query(apiName, c.Request.Header)
		if !exist {
			c.JSON(
				http.StatusOK,
				&PostAPIResponse{
					ErrorNamespace: "POST-API",
					Code:           http.StatusNotFound,
					Message:        fmt.Sprintf("Api Not Found: %s", apiName),
				},
			)
			return
		}

		preperReqs[apiName] = jsonData
	}

	if len(preperReqs) == 0 {
		c.JSON(http.StatusOK, PostAPIResponse{})
		return
	}

	strTimeout := c.GetHeader(XApiTimeout)
	timeout := time.Second * 30

	if len(strTimeout) > 0 {
		if dur, e := time.ParseDuration(strTimeout); e == nil {
			timeout = dur
		}
	}

	wg := &sync.WaitGroup{}

	respChan := make(chan *PostAPIResponse, len(preperReqs))

	wg.Add(len(preperReqs))
	for apiName, data := range preperReqs {
		go func(apiName string, data []byte, timeout time.Duration, c *gin.Context, respC chan<- *PostAPIResponse) {

			defer wg.Done()
			resp := p.call(apiName, data, timeout, c)
			respC <- resp

		}(apiName, data, timeout, c, respChan)
	}

	wg.Wait()
	close(respChan)

	resps := map[string]*PostAPIResponse{}
	for resp := range respChan {
		resps[resp.API] = resp
	}

	rawResp, err := json.Marshal(resps)
	if err != nil {
		return
	}

	c.JSON(http.StatusOK,
		PostAPIResponse{
			Result: rawResp,
		},
	)

	return
}

func (p *PostAPI) serveSingleCall(c *gin.Context) (err error) {

	var body []byte
	body, err = c.GetRawData()

	if err != nil {
		return
	}

	apiName := c.GetHeader(XApi)
	strTimeout := c.GetHeader(XApiTimeout)
	timeout := time.Second * 30

	if len(strTimeout) > 0 {
		if dur, e := time.ParseDuration(strTimeout); e == nil {
			timeout = dur
		}
	}

	resp := p.call(apiName, body, timeout, c)

	c.JSON(http.StatusOK, resp)

	return
}

func (p *PostAPI) callback(session mail.Session) (err error) {
	fbp.BreakSession(session)

	cacheKey := p.cacheKey(session.Payload().ID())
	itemV, exist := p.opts.Cache.Get(cacheKey)
	if !exist {
		err = fmt.Errorf("cache is dropped, key: %s", cacheKey)
		return
	}

	defer p.opts.Cache.Delete(cacheKey)
	// should add session id to cache
	item, ok := itemV.(*httpCacheItem)
	if !ok {
		err = errors.New("http component handler could not get response object")
		return
	}

	if item == nil || item.done == nil || item.ctx == nil {
		return
	}

	payload, ok := session.Payload().Interface().(*protocol.Payload)
	if !ok {
		err = errors.New("could not convert session payload to *protocol.Payload")
		return
	}

	if item.ctx.Err() != nil {
		return
	}

	apiResp := &PostAPIResponse{}
	msgErr := payload.GetMessage().GetErr()
	if msgErr != nil {
		apiResp.Code = msgErr.GetCode()
		apiResp.Message = msgErr.GetDescription()
		apiResp.ErrorNamespace = msgErr.GetNamespace()
	} else {
		apiResp.Code = 0
		apiResp.Result = payload.GetMessage().GetBody()
	}

	item.done <- apiResp

	return
}

func (p *PostAPI) Route(mail.Session) worker.HandlerFunc {
	return p.callback
}

func (p *PostAPI) Start() error {

	go func() {

		var err error

		sslConf := p.opts.Config.GetConfig("http.ssl")

		if sslConf != nil && sslConf.GetBoolean("enabled", false) {
			err = p.srv.ListenAndServeTLS(
				sslConf.GetString("cert-file"),
				sslConf.GetString("cert-key"),
			)
		} else {
			err = p.srv.ListenAndServe()
		}

		if err != http.ErrServerClosed {
			logrus.WithField("component", "post-api").WithField("alias", p.alias).WithError(err).Errorln("Listen")
		}
	}()
	return nil
}

func (p *PostAPI) Stop() error {

	dur := p.opts.Config.GetTimeDuration("http.shutdown-timeout", time.Second*30)

	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()

	if err := p.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("postapi shutdown failure, err: %s", err)
	}

	logrus.WithField("component", "post-api").WithField("alias", p.alias).Infoln("Server exiting")

	return nil
}

func (p *PostAPI) Alias() string {
	if p == nil {
		return ""
	}
	return p.alias
}

func (p *PostAPI) Document() doc.Document {
	document := doc.Document{
		Title:       "Post API is an gateway for user request, it provide API to Graph's mapping",
		Description: "",
	}

	return document
}

func (p *PostAPI) cacheKey(id string) string {
	return fmt.Sprintf("POSTAPI:REQ:%s", id)
}
