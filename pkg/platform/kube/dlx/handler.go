package dlx

import "github.com/valyala/fasthttp"

type Handler struct {
	requestHandler fasthttp.RequestHandler
	functionStarter *FunctionStarter
}

func NewHandler(functionStarter *FunctionStarter) (Handler, error) {
	h := Handler{
		functionStarter: functionStarter,
	}
	h.requestHandler = h.handleRequest
	return h, nil
}

func (h *Handler) handleRequest(ctx *fasthttp.RequestCtx) {
	responseChannel := make(chan *fasthttp.Response, 1)
	h.functionStarter.SendRequestGetResponse(&ctx.Request, responseChannel)
	response := <- responseChannel
	ctx.Response = *response
}
