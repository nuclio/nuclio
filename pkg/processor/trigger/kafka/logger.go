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

package kafka

import (
	"strings"

	"github.com/nuclio/logger"
)

type SaramaLogger struct {
	logger logger.Logger
}

func NewSaramaLogger(parentLogger logger.Logger) SaramaLogger {
	return SaramaLogger{parentLogger.GetChild("sarama")}
}

func (s SaramaLogger) Print(v ...interface{}) {
	format := strings.Repeat("%v", len(v))
	s.output(format, v...)
}

func (s SaramaLogger) Printf(format string, v ...interface{}) {
	s.output(format, v...)
}

func (s SaramaLogger) Println(v ...interface{}) {
	format := strings.Repeat("%v", len(v))
	s.output(format, v...)
}

func (s SaramaLogger) output(format string, v ...interface{}) {
	s.logger.Debug("Sarama: "+strings.TrimSuffix(format, "\n"), v...)
}
