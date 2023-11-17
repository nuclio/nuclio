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

package runtimeconfig

import (
	"os"

	"github.com/nuclio/errors"
	"k8s.io/api/core/v1"
)

type Config struct {
	Common *Common `json:"common,omitempty"`
	Python *Python `json:"python,omitempty"`
}

type Common struct {
	Env       map[string]string `json:"env,omitempty"`
	BuildArgs map[string]string `json:"buildArgs,omitempty"`
	EnvFrom   v1.EnvFromSource  `json:"envFrom,omitempty"`
}

type Python struct {
	Common `json:",inline"`

	PipCAPath     string `json:"pipCAPath,omitempty"`
	pipCAContents []byte
}

// GetPipCAContents lazy reads and stores pip-ca file contents
func (p *Python) GetPipCAContents() ([]byte, error) {
	if p.PipCAPath == "" {
		return nil, nil
	}

	if len(p.pipCAContents) > 0 {
		return p.pipCAContents, nil
	}

	pipCaContents, err := os.ReadFile(p.PipCAPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read pip ca file contents")
	}

	if len(pipCaContents) == 0 {
		return nil, errors.New("Pip CA file contents is empty")
	}
	p.pipCAContents = pipCaContents
	return p.pipCAContents, nil
}
