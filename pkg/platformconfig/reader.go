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

import (
	"io"
	"io/ioutil"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/ghodss/yaml"
)

type Reader struct {
}

func NewReader() (*Reader, error) {
	return &Reader{}, nil
}

func (r *Reader) Read(reader io.Reader, configType string, config *Configuration) error {
	configBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return errors.Wrap(err, "Failed to read platform configuration")
	}

	return yaml.Unmarshal(configBytes, config)
}
