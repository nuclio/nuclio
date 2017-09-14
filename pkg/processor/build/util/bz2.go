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
	"compress/bzip2"
	"io"
	"os"
)

// BZ2Decompress decompressed a bz2 file in srcPath to destPath
func BZ2Decompress(srcPath, destPath string) error {
	inFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}

	defer inFile.Close()
	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}

	defer outFile.Close()

	reader := bzip2.NewReader(inFile)
	_, err = io.Copy(outFile, reader)
	if err != nil {
		return err
	}

	return outFile.Close()
}
