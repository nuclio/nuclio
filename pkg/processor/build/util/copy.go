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
	"path/filepath"
)

// CopyFile copies file source to destination dest.
func CopyFile(source string, dest string) error {
	sf, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sf.Close()
	df, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer df.Close()
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
	// get properties of source dir
	fi, err := os.Stat(source)
	if err != nil {
		return false, nil
	}

	if !fi.IsDir() {
		return false, fmt.Errorf("Source (%q) is not a directory", source)
	}

	// ensure dest dir does not already exist

	//_, err = os.Open(dest)
	//if !os.IsNotExist(err) {
	//	return false, fmt.Errorf("Destination already exists: %q", dest)
	//}

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
