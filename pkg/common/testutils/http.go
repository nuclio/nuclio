/*
Copyright 2023 The Nuclio Authors.

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

package testutils

import (
	"net/http"

	"github.com/nuclio/errors"
)

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	response := f(req)
	if response == nil {
		return nil, errors.New("EOF")
	}
	return response, nil
}

func newHTTPClient(fn roundTripFunc) *http.Client { // nolint: interfacer
	return &http.Client{
		Transport: fn,
	}
}

func CreateDummyHTTPClient(handler func(r *http.Request) *http.Response) *http.Client {
	return newHTTPClient(func(req *http.Request) *http.Response {
		return handler(req)
	})
}
