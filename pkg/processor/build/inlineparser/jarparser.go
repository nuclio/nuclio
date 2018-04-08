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

package inlineparser

import (
	"archive/zip"
	"io/ioutil"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/logger"
	"gopkg.in/yaml.v2"
)

// JarParser parses config in Jar files
type JarParser struct {
	logger logger.Logger
}

// NewJarParser returns a new Jar config parser
func NewJarParser(logger logger.Logger) *JarParser {
	return &JarParser{logger}
}

// Parse parses config in Jar file
func (j *JarParser) Parse(path string) (map[string]map[string]interface{}, error) {
	j.logger.DebugWith("Processing jar for configuration files", "path", path)
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}

	defer zipReader.Close() // nolint: errcheck
	config := make(map[string]interface{})

	for _, zipInfo := range zipReader.File {
		if !j.isConfigFile(zipInfo) {
			continue
		}
		j.logger.DebugWith("Found config file", "name", zipInfo.Name)

		file, err := zipInfo.Open()
		if err != nil {
			return nil, errors.Wrapf(err, "Can't open zip entry", "name", zipInfo.Name)
		}

		defer file.Close() // nolint: errcheck
		data, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, errors.Wrapf(err, "Can't read zip entry", "name", zipInfo.Name)
		}

		configSection := map[string]interface{}{}
		if err := yaml.Unmarshal(data, &configSection); err != nil {
			return nil, errors.Wrapf(err, "Can't unmarshal", "name", zipInfo.Name)
		}
		config[zipInfo.Name] = configSection
	}

	outer := make(map[string]map[string]interface{})
	outer["configure"] = config

	return outer, nil
}

func (j *JarParser) isConfigFile(zipFile *zip.File) bool {
	if zipFile.FileInfo().IsDir() {
		return false
	}

	if !strings.HasSuffix(zipFile.Name, ".yaml") {
		return false
	}

	return true
}
