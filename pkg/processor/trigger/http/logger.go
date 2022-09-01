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

package http

import (
	"strings"

	"github.com/nuclio/logger"
)

type FastHTTPLogger struct {
	logger logger.Logger
}

func NewFastHTTPLogger(parentLogger logger.Logger) FastHTTPLogger {
	return FastHTTPLogger{parentLogger.GetChild("fasthttp")}
}

func (s FastHTTPLogger) Printf(format string, args ...interface{}) {
	s.output(format, args...)
}

func (s FastHTTPLogger) output(format string, args ...interface{}) {
	s.logger.Debug("FastHTTP: "+strings.TrimSuffix(format, "\n"), args...)
}
