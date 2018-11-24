// Copyright 2015 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package header

import (
	"net"
	"net/http"

	"github.com/google/martian"
)

// NewForwardedModifier sets the X-Forwarded-For, X-Forwarded-Proto, and
// X-Forwarded-Host headers.
//
// If X-Forwarded-For is already present, the client IP is appended to
// the existing value.
//
// TODO: Support "Forwarded" header.
// see: http://tools.ietf.org/html/rfc7239
func NewForwardedModifier() martian.RequestModifier {
	return martian.RequestModifierFunc(
		func(req *http.Request) error {
			req.Header.Set("X-Forwarded-Proto", req.URL.Scheme)
			req.Header.Set("X-Forwarded-Host", req.Host)
			req.Header.Set("X-Forwarded-Url", req.URL.String())

			xff, _, err := net.SplitHostPort(req.RemoteAddr)
			if err != nil {
				xff = req.RemoteAddr
			}

			if v := req.Header.Get("X-Forwarded-For"); v != "" {
				xff = v + ", " + xff
			}

			req.Header.Set("X-Forwarded-For", xff)

			return nil
		})
}
