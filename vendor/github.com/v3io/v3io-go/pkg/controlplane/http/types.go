/*
Copyright 2018 The v3io Authors.

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

package v3iochttp

import (
	"github.com/valyala/fasthttp"
)

type request struct {
	method      string
	path        string
	headers     map[string]string
	httpRequest *fasthttp.Request
	allowErrors bool
}

type response struct {
	statusCode int
	headers    map[string]string
	body       []byte
	cookies    map[string]string
}

type jsonapiData struct {
	ID         interface{} `json:"id,omitempty"`
	Type       string      `json:"type,omitempty"`
	Attributes interface{} `json:"attributes,omitempty"`
}

type jsonapiResource struct {
	Data jsonapiData `json:"data,omitempty"`
}
