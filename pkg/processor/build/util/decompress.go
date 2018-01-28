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

package util

import (
	"fmt"
	"path"
	"reflect"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/mholt/archiver"
	"github.com/nuclio/logger"
)

type Decompressor struct {
	logger logger.Logger
}

func NewDecompressor(parentLogger logger.Logger) (*Decompressor, error) {
	newDecompressor := &Decompressor{
		logger: parentLogger,
	}

	return newDecompressor, nil
}

func (d *Decompressor) Decompress(source string, target string) error {
	fileArchiver := archiver.MatchingFormat(source)
	if fileArchiver == nil {
		return fmt.Errorf("File %s is not compressed or has an unknown extension", source)
	}

	d.logger.DebugWith("File is compressed, now decompressing",
		"file", source,
		"compression", reflect.TypeOf(fileArchiver),
		"target", target)

	if err := fileArchiver.Open(source, target); err != nil {
		return errors.Wrapf(err, "Failed to decompress file %s", source)
	}

	return nil
}

func IsCompressed(source string) bool {
	// Jars are special case
	if strings.ToLower(path.Ext(source)) == ".jar" {
		return false
	}

	fileArchiver := archiver.MatchingFormat(source)
	return fileArchiver != nil
}
