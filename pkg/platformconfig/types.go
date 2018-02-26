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

package platformconfig

type LoggerSink struct {
	Kind       string                 `json:"kind,omitempty"`
	URL        string                 `json:"url,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type LoggerSinkWithLevel struct {
	Level string
	Sink  LoggerSink
}

type LoggerSinkBinding struct {
	Level string `json:"level,omitempty"`
	Sink  string `json:"sink,omitempty"`
}

type FunctionsLogger struct {
	DefaultLevel string `json:"defaultLevel,omitempty"`
	DefaultSink  string `json:"defaultSink,omitempty"`
}

type Logger struct {
	Sinks     map[string]LoggerSink `json:"sinks,omitempty"`
	System    []LoggerSinkBinding   `json:"system,omitempty"`
	Functions []LoggerSinkBinding   `json:"functions,omitempty"`
}

type WebServer struct {
	Enabled       *bool  `json:"enabled,omitempty"`
	ListenAddress string `json:"listenAddress,omitempty"`
}

type MetricSink struct {
	Enabled    *bool                  `json:"enabled,omitempty"`
	Kind       string                 `json:"kind,omitempty"`
	URL        string                 `json:"url,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type Metrics struct {
	Sinks     map[string]MetricSink `json:"sinks,omitempty"`
	System    []string              `json:"system,omitempty"`
	Functions []string              `json:"functions,omitempty"`
}
