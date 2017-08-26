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

// Copies file source to destination dest.
func CopyFile(source string, dest string) (err error) {
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
	_, err = io.Copy(df, sf)
	if err == nil {
		var si os.FileInfo

		si, err = sf.Stat()
		if err == nil {
			err = os.Chmod(dest, si.Mode())
		}
	}

	return
}

// Recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
func CopyDir(source string, dest string) (err error) {

	// get properties of source dir
	fi, err := os.Stat(source)
	if err != nil {
		return
	}

	if !fi.IsDir() {
		return &CustomError{"Source is not a directory"}
	}

	// ensure dest dir does not already exist

	_, err = os.Open(dest)
	if !os.IsNotExist(err) {
		return &CustomError{fmt.Sprintf("Destination already exists: %s", dest)}
	}

	// create dest dir

	err = os.MkdirAll(dest, fi.Mode())
	if err != nil {
		return
	}

	entries, err := ioutil.ReadDir(source)
	if err != nil {
		return
	}
	for _, entry := range entries {

		sfp := filepath.Join(source, entry.Name())
		dfp := filepath.Join(dest, entry.Name())
		if entry.IsDir() {
			err = CopyDir(sfp, dfp)
			if err != nil {
				return
			}
		} else {
			// perform copy
			err = CopyFile(sfp, dfp)
			if err != nil {
				return
			}
		}

	}
	return
}

// A struct for returning custom error messages
type CustomError struct {
	What string
}

// Returns the error message defined in What as a string
func (e *CustomError) Error() string {
	return e.What
}
