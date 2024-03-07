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

package util

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v4"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type Unarchiver struct {
	logger logger.Logger
}

func NewUnarchiver(parentLogger logger.Logger) (*Unarchiver, error) {
	newUnarchiver := &Unarchiver{
		logger: parentLogger,
	}

	return newUnarchiver, nil
}

// Extract extracts the source archive to the target path
func (d *Unarchiver) Extract(ctx context.Context, sourcePath string, targetPath string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to open archive file: %s", sourcePath))
	}

	// identify the format of the archive
	format, input, err := archiver.Identify(sourcePath, file)
	if err != nil {
		return errors.Wrap(err, "Failed to identify archive")
	}

	extractionHandler := func(ctx context.Context, file archiver.File) error {
		if filterFile, err := d.filterFile(file, targetPath); err != nil {
			return errors.Wrap(err, "Failed to filter archived file")
		} else if filterFile {
			return nil
		}

		// copy the file to a new file with the same name in the target path
		return d.extractFile(file, targetPath)
	}

	if extractor, ok := format.(archiver.Extractor); ok {
		if err := extractor.Extract(ctx, input, nil, extractionHandler); err != nil {
			return errors.Wrap(err, "Failed to extract archive")
		}
	}

	return nil
}

// extractFile extracts a file from the archive to the target path with the same base name
func (d *Unarchiver) extractFile(file archiver.File, targetPath string) error {
	filePath := filepath.Join(targetPath, file.NameInArchive)

	// create the directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to create directory %s", filepath.Dir(filePath)))
	}

	// create the new file
	outFile, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to create file :%s", filePath))
	}
	defer outFile.Close() // nolint: errcheck

	// set the file mode
	err = outFile.Chmod(file.Mode())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to change file mode for %s", filePath))
	}

	// open the file
	input, err := file.Open()
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to open file: %s", filePath))
	}

	// copy the file contents
	_, err = io.Copy(outFile, input)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to write file: %s", filePath))
	}
	return nil
}

// filterFile checks if the file should be extracted or not by checking if the destination file path or resolved
// linked path is outside the target directory
func (d *Unarchiver) filterFile(file archiver.File, targetPath string) (bool, error) {
	for _, pathToCheck := range []string{
		file.NameInArchive,
		file.LinkTarget,
	} {
		if pathToCheck == "" {
			continue
		}
		fullPath, err := filepath.Abs(path.Join(targetPath, pathToCheck))
		if err != nil {
			return true, errors.Wrap(err, fmt.Sprintf("Failed to get absolute path for %s", fullPath))
		}
		if !strings.HasPrefix(fullPath, targetPath) {
			return true, nil
		}
	}

	return false, nil
}

// IsArchive checks if the file is an archive
func IsArchive(source string) bool {

	// Jars are special case
	if IsJar(source) {
		return false
	}

	file, err := os.Open(source)
	if err != nil {
		// if we can't open the file, it's not compressed
		return false
	}

	// check if the file is an archive
	format, _, err := archiver.Identify(source, file)
	if err != nil {
		return false
	}

	// if the format is nil, it's not an archive
	if format == nil {
		return false
	}

	// if the format is an extractor, it's an archive
	_, ok := format.(archiver.Extractor)
	return ok
}

// IsJar checks if the file is a jar
func IsJar(source string) bool {
	return strings.ToLower(path.Ext(source)) == ".jar"
}
