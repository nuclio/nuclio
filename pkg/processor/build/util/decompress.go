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

type Decompressor struct {
	logger logger.Logger
}

func NewDecompressor(parentLogger logger.Logger) (*Decompressor, error) {
	newDecompressor := &Decompressor{
		logger: parentLogger,
	}

	return newDecompressor, nil
}

func (d *Decompressor) Decompress(ctx context.Context, source string, target string) error {
	if err := d.ExtractArchive(ctx, source, target); err != nil {
		return errors.Wrapf(err, "Failed to extract archive %s", source)
	}

	return nil
}

func (d *Decompressor) ExtractArchive(ctx context.Context, sourcePath string, targetPath string) error {
	d.logger.DebugWith("Extracting archive", "sourcePath", sourcePath, "targetPath", targetPath)

	// open the source archive
	file, err := os.Open(sourcePath)
	if err != nil {
		return errors.Wrap(err, "Failed to open file")
	}

	d.logger.DebugWith("Opened file, identifying it", "sourcePath", sourcePath, "file", file)

	// identify the type of the archive
	format, input, err := archiver.Identify(sourcePath, file)
	if err != nil {
		return errors.Wrap(err, "Failed to identify archive")
	}

	d.logger.DebugWith("Identified archive", "format", format, "input", input)

	handler := func(ctx context.Context, file archiver.File) error {
		d.logger.DebugWith("Extracting file", "sourcePath", file.Name, "targetPath", targetPath)

		if filterFile, err := d.filterArchivedFile(file, targetPath); err != nil {
			return errors.Wrap(err, "Failed to filter archived file")
		} else if filterFile {
			return nil
		}

		// copy the file to a new file with the same name in the target path
		return d.extractArchivedFile(file, targetPath)
	}

	// want to extract something?
	if extractor, ok := format.(archiver.Extractor); ok {
		d.logger.DebugWith("Extracting archive", "sourcePath", sourcePath, "targetPath", targetPath, "extractor", extractor)
		if err := extractor.Extract(ctx, input, nil, handler); err != nil {
			return errors.Wrap(err, "Failed to extract archive")
		}
	}

	return nil
}

func (d *Decompressor) extractArchivedFile(file archiver.File, targetPath string) error {
	filePath := filepath.Join(targetPath, file.NameInArchive)

	// create the directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to create directory %s", filepath.Dir(filePath)))
	}

	// create the new file
	outFile, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed creating file :%s", filePath))
	}
	defer outFile.Close() // nolint: errcheck

	// set the file mode
	err = outFile.Chmod(file.Mode())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed changing file mode for %s", filePath))
	}

	// open the file
	input, err := file.Open()
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed opening file: %s", filePath))
	}

	// copy the file contents
	_, err = io.Copy(outFile, input)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed writing file: %s", filePath))
	}
	return nil
}

func (d *Decompressor) filterArchivedFile(file archiver.File, targetPath string) (bool, error) {
	// filter symlinks that point outside the archive or to an absolute path
	if file.LinkTarget != "" && (strings.HasPrefix(file.LinkTarget, "..") || filepath.IsAbs(file.LinkTarget)) {
		return true, nil
	}

	// check that the destination file path is not outside the target directory
	fullTargetPath, err := filepath.Abs(path.Join(targetPath, file.NameInArchive))
	if err != nil {
		return true, errors.Wrap(err, fmt.Sprintf("Failed to get absolute path for %s", fullTargetPath))
	}
	if !strings.HasPrefix(fullTargetPath, targetPath) {
		return true, nil
	}

	// filter files that their name is an absolute path
	if filepath.IsAbs(file.NameInArchive) {
		return true, nil
	}

	// filter files that their name contains ".."
	if strings.HasPrefix(file.NameInArchive, "..") || strings.Contains(file.NameInArchive, "/..") {
		return true, nil
	}

	// all good
	return false, nil
}

func IsCompressed(source string) bool {

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

func IsJar(source string) bool {
	return strings.ToLower(path.Ext(source)) == ".jar"
}
