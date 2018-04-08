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
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
)

// CopyTo copies source to dest
// If source is a file and dest is a file, performs copy file
// If source is a dir and dest is a dir, performs copy dir
// If source is a file and dest is a dir, copies source to the dest dir with the same name
// If source is a dir and dest is a file, returns error
func CopyTo(source string, dest string) error {
	sourceIsFile := common.IsFile(source)
	destIsFile := common.IsFile(dest)

	// dir -> file
	if !sourceIsFile && destIsFile {
		return errors.New("Cannot copy directory to file")
	}

	// file -> file
	if sourceIsFile && destIsFile {
		return CopyFile(source, dest)
	}

	// dir -> dir
	if !sourceIsFile && !destIsFile {
		_, err := CopyDir(source, dest)
		return err
	}

	// file -> dir
	if sourceIsFile && !destIsFile {

		// get the source file name
		sourceFileName := path.Base(source)

		// dest is dest dir + source file name
		dest = path.Join(dest, sourceFileName)

		// get the destination as the dest dir +
		return CopyFile(source, dest)
	}

	return errors.New("Should not get here")
}

// CopyFile copies file source to destination dest.
func CopyFile(source string, dest string) error {
	sf, err := os.Open(source)
	if err != nil {
		return err
	}

	defer sf.Close() // nolint: errcheck

	df, err := os.Create(dest)
	if err != nil {
		return err
	}

	defer df.Close() // nolint: errcheck

	if _, err = io.Copy(df, sf); err != nil {
		return err
	}

	si, err := sf.Stat()
	if err == nil {
		return os.Chmod(dest, si.Mode())
	}

	return nil
}

// CopyDir Recursively copies a directory tree, attempting to preserve
// permissions.  Source directory must exist, destination directory must *not*
// exist.
func CopyDir(source string, dest string) (bool, error) {

	if source == dest {
		return true, nil
	}
	// get properties of source dir
	fi, err := os.Stat(source)
	if err != nil {
		return false, nil
	}

	if !fi.IsDir() {
		return false, fmt.Errorf("Source (%q) is not a directory", source)
	}

	// create dest dir
	err = os.MkdirAll(dest, fi.Mode())
	if err != nil {
		return false, err
	}

	entries, err := ioutil.ReadDir(source)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		sfp := filepath.Join(source, entry.Name())
		dfp := filepath.Join(dest, entry.Name())
		if entry.IsDir() {
			_, err = CopyDir(sfp, dfp)
			if err != nil {
				return false, err
			}
		} else {
			// perform copy
			err = CopyFile(sfp, dfp)
			if err != nil {
				return false, err
			}
		}

	}
	return true, nil
}
