package postapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"

	"github.com/go-spirit/spirit/component"
	"github.com/go-spirit/spirit/doc"
	"github.com/go-spirit/spirit/mail"
	"github.com/go-spirit/spirit/message"
	"github.com/go-spirit/spirit/protocol"
	"github.com/go-spirit/spirit/worker"
	"github.com/go-spirit/spirit/worker/fbp"
)

type ctxHttpComponentKey struct{}

type httpCacheItem struct {
	c    *gin.Context
	ctx  context.Context
	done chan struct{}
}

type PostAPIResponse struct {
	Code           int64           `json:"code"`
	ErrorId        string          `json:"error_id,omitempty"`
	ErrorNamespace string          `json:"error_namespace,omitempty"`
	Message        string          `json:"message,omitempty"`
	Result         json.RawMessage `json:"result,omitempty"`
}

type PostAPI struct {
	opts component.Options

	graphs GraphProvider

	router *gin.Engine
}

func init() {
	component.RegisterComponent("post-api", NewPostAPI)
	doc.RegisterDocumenter("post-api", &PostAPI{})
}

func NewPostAPI(opts ...component.Option) (srv component.Component, err error) {
	s := &PostAPI{}

	s.init(opts...)

	srv = s
	return
}

func (p *PostAPI) init(opts ...component.Option) (err error) {

	for _, o := range opts {
		o(&p.opts)
	}

	p.graphs, err = newDefaultGraphProvider(
		p.opts.Config.GetConfig("api"),
	)

	if err != nil {
		return
	}

	debug := p.opts.Config.GetBoolean("debug", false)
	if !debug {
		gin.SetMode("release")
	}

	urlPath := p.opts.Config.GetString("path", "/")

	router := gin.New()
	router.Use(gin.Recovery())
	router.POST(urlPath, p.serve)

	p.router = router

	return
}

func (p *PostAPI) serve(c *gin.Context) {

	var err error

	var body []byte
	body, err = c.GetRawData()

	if err != nil {
		return
	}

	apiName := c.GetHeader("X-Api")

	fmt.Println(string(body))

	graph, exist := p.graphs.Query(apiName)

	if !exist {
		c.JSON(http.StatusNotFound,
			PostAPIResponse{
				ErrorNamespace: "POST-API",
				Code:           http.StatusNotFound,
				Message:        "Api Not Found",
			},
		)
		return
	}

	id := uuid.NewUUID()
	payload := &protocol.Payload{
		Id:        id.String(),
		Timestamp: time.Now().UnixNano(),
		Graph: &protocol.Graph{
			Seq:    1,
			Errors: graph.Errors,
			Ports:  graph.Ports,
		},
		Message: &protocol.Message{
			Id:     id.String(),
			Header: map[string]string{"content-type": "application/json"},
			Body:   body,
		},
	}

	port, err := payload.GetGraph().CurrentPort()

	if err != nil {
		return
	}

	session := mail.NewSession()

	session.WithPayload(payload)
	session.WithFromTo("", port.GetUrl())

	fbp.SessionWithPort(session, port.GetUrl(), false, port.GetMetadata())

	var ctx context.Context
	var cancel context.CancelFunc
	var doneChan chan struct{}

	// if wait {

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	doneChan = make(chan struct{})
	defer close(doneChan)

	session.WithValue(ctxHttpComponentKey{}, &httpCacheItem{c, ctx, doneChan})

	// } else {
	// 	session.WithValue(ctxHttpComponentKey{}, &httpCacheItem{c, nil, nil})
	// }

	err = p.opts.Postman.Post(
		message.NewUserMessage(session),
	)

	if err != nil {

		logrus.WithField("component", "post-api").
			WithField("from", session.From()).
			WithField("to", session.To()).
			WithField("seq", payload.GetGraph().GetSeq()).
			WithError(err).Errorln("post user message failure")
		return
	}

	// if !wait {
	// 	c.JSON(http.StatusOK, struct{}{})
	// 	return
	// }

	for {
		select {
		case <-doneChan:
			{
				return
			}
		case <-ctx.Done():
			{
				c.JSON(http.StatusRequestTimeout, PostAPIResponse{
					ErrorNamespace: "POST-API",
					Code:           http.StatusRequestTimeout,
					Message:        "Request Timeout",
				})
				return
			}
		}
	}

}

func (p *PostAPI) Handler() worker.HandlerFunc {
	return func(session mail.Session) (err error) {

		fbp.BreakSession(session)
		fmt.Print("break")

		item, ok := session.Value(ctxHttpComponentKey{}).(*httpCacheItem)
		if !ok {
			err = errors.New("http component handler could not get response object")
			return
		}

		if item.done == nil || item.ctx == nil {
			return
		}

		payload, ok := session.Payload().(*protocol.Payload)
		if !ok {
			err = errors.New("could not convert session payload to *protocol.Payload")
			return
		}

		if item.ctx.Err() != nil {
			return
		}

		var apiResp PostAPIResponse
		msgErr := payload.GetMessage().GetError()
		if msgErr != nil {
			apiResp.Code = msgErr.GetCode()
			apiResp.Message = msgErr.GetDescription()
			apiResp.ErrorNamespace = msgErr.GetNamespace()
		} else {
			apiResp.Code = 0
			apiResp.Result = payload.GetMessage().GetBody()
		}

		item.c.JSON(http.StatusOK, apiResp)

		item.done <- struct{}{}

		return
	}
}

func (p *PostAPI) Start() error {
	go p.router.Run()
	return nil
}

func (p *PostAPI) Stop() error {
	return nil
}

func (p *PostAPI) Document() doc.Document {
	document := doc.Document{
		Title:       "Post API is an gateway for user request, it provide API to Graph's mapping",
		Description: "",
	}

	return document
}