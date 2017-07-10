package http

import (
	"github.com/nuclio/nuclio-sdk/event"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/valyala/fasthttp"
)

// allows accessing fasthttp.RequestCtx as a event.Sync
type Event struct {
	event.AbstractSync
	ctx *fasthttp.RequestCtx
}

func (e *Event) GetContentType() string {
	return common.ByteArrayToString(e.ctx.Request.Header.ContentType())
}

func (e *Event) GetBody() []byte {
	return e.ctx.Request.Body()
}

func (e *Event) GetHeaderByteSlice(key string) []byte {

	// TODO: copy underlying by default? huge gotcha
	return e.ctx.Request.Header.Peek(key)
}
