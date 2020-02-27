package v3iohttp

import "github.com/valyala/fasthttp"

type NewContextInput struct {
	HTTPClient     *fasthttp.Client
	NumWorkers     int
	RequestChanLen int
}
