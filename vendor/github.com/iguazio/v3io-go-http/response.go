package v3io

import "github.com/valyala/fasthttp"

type Response struct {
	response *fasthttp.Response

	// hold a decoded output, if any
	Output interface{}
}

func (r *Response) Release() {
	fasthttp.ReleaseResponse(r.response)
}

func (r *Response) Body() []byte {
	return r.response.Body()
}
