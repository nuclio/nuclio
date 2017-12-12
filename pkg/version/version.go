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

package version

import (
	"encoding/json"
	"io/ioutil"

	"github.com/nuclio/nuclio-sdk"
)

type Info struct {
	Label     string `json:"label"`
	GitCommit string `json:"git_commit"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	GoVersion string `json:"go_version"`
}

// these global variables are initialized by the build process if the build target
// is a standalone binary (e.g. nuctl)
var (
	label     = ""
	gitCommit = ""
	os        = ""
	arch      = ""
	goVersion = ""
)

var info Info

// Get returns the version information
func Get() (*Info, error) {

	// if the label is empty, try to read the version file @ a static path
	if label == "" {

		// no need to read the file more than once
		if info.Label == "" {

			// try to read. if failed, not
			if err := readVersionFile(&info); err != nil {
				return nil, err
			}
		}

		// return the version info from file
		return &info, nil
	}

	// return the info initialized by the linker during build
	return &Info{
		Label:     label,
		GitCommit: gitCommit,
		OS:        os,
		Arch:      arch,
		GoVersion: goVersion,
	}, nil
}

// Set will update the stored version info, used primarily for tests
func Set(info *Info) error {
	label = info.Label
	gitCommit = info.GitCommit
	os = info.OS
	arch = info.Arch
	goVersion = info.GoVersion

	return nil
}

// Log will log the version, or an error
func Log(logger nuclio.Logger) {
	versionInfo, err := Get()
	if err != nil {
		logger.WarnWith("Failed to read version info", "err", err)
		return
	}

	logger.InfoWith("Read version", "version", *versionInfo)
}

func readVersionFile(versionInfo *Info) error {
	versionFileContents, err := ioutil.ReadFile("/etc/nuclio/version_info.json")
	if err != nil {
		return err
	}

	// parse the file
	if err := json.Unmarshal(versionFileContents, versionInfo); err != nil {
		return err
	}

	return nil
}
