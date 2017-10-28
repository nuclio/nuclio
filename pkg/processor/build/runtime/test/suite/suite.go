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

package buildsuite

import (
	"net/http"
	"path"

	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
)

type TestSuite struct {
	httpsuite.TestSuite
}

func (suite *TestSuite) GetProcessorBuildDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "build", "runtime")
}

//
// HTTP server to test URL fetch
//

type HTTPFileServer struct {
	http.Server
}

func (hfs *HTTPFileServer) Start(addr string, localPath string, pattern string) {
	hfs.Addr = addr

	http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, localPath)
	})

	go hfs.ListenAndServe()
}
