/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package runtime

import (
	"fmt"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/valyala/fasthttp"
)

type Platform struct {
	client    fasthttp.Client
	logger    logger.Logger
	kind      string
	namespace string
}

func NewPlatform(parentLogger logger.Logger, kind string, namespace string) (*Platform, error) {
	return &Platform{
		client:    fasthttp.Client{},
		logger:    parentLogger.GetChild("platform"),
		kind:      kind,
		namespace: namespace,
	}, nil
}

func (p *Platform) getFunctionHost(name string) string {
	if p.kind == "local" {
		return fmt.Sprintf("%s-%s:8080", p.namespace, name)
	}
	return fmt.Sprintf("%s:8080", name)
}

func (p *Platform) createRequest(functionName string, event nuclio.FunctionCallEvent) *fasthttp.Request {
	req := fasthttp.AcquireRequest()
	req.URI().SetScheme("http")
	req.URI().SetHost(p.getFunctionHost(functionName))
	req.URI().SetPath(event.GetPath())
	req.SetBody(event.GetBody())
	req.Header.SetContentType(event.GetContentType())
	req.Header.SetMethod(event.GetMethod())
	return req
}

func (p *Platform) createResponse(resp *fasthttp.Response) nuclio.Response {
	result := nuclio.Response{}
	if len(resp.Header.ContentType()) == 0 {
		result.ContentType = "text/plain"
	} else {
		result.ContentType = string(resp.Header.ContentType())
	}

	result.StatusCode = resp.StatusCode()

	result.Headers = make(map[string]interface{})
	resp.Header.VisitAll(func(key, value []byte) {
		result.Headers[string(key)] = string(value)
	})

	result.Body = append(result.Body, resp.Body()...)

	return result
}

func (p *Platform) CallFunction(functionName string, event nuclio.FunctionCallEvent) (nuclio.Response, error) {
	p.logger.DebugWith("Calling", "function", functionName)

	req := p.createRequest(functionName, event)

	p.logger.DebugWith("Sending", "request", req.String())

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err := p.client.Do(req, resp)
	fasthttp.ReleaseRequest(req)
	if err != nil {
		p.logger.DebugWith("Function call failed", "error", err)
		return nuclio.Response{}, err
	}

	return p.createResponse(resp), nil
}
