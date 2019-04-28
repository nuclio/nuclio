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

package common

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func DownloadFile(URL, destFile string, headers http.Header) error {
	out, err := os.Create(destFile)
	if err != nil {
		return err
	}

	client := http.Client{}
	request, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return err
	}

	request.Header = headers
	response, err := client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close() // nolint: errcheck

	written, err := io.Copy(out, response.Body)

	if err != nil {
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	if response.ContentLength != -1 && written != response.ContentLength {
		return fmt.Errorf(
			"Downloaded file length (%d) is different than URL content length (%d)",
			written,
			response.ContentLength)
	}

	return nil
}

func IsURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func IsLocalFileURL(s string) bool {
	return strings.HasPrefix(s, "file://")
}

// extracts absolute path to file from local file URL
// example: "file://path/to/file" -> "/path/to/file"
func GetPathFromLocalFileURL(s string) string {
	if IsLocalFileURL(s) {
		return s[6:]
	}
	return ""
}
