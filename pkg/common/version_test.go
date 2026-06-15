//go:build test_unit

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

package common

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/v3io/version-go"
)

type VersionTestSuite struct {
	suite.Suite
	originalVersion *version.Info
}

func (suite *VersionTestSuite) SetupSuite() {
	suite.originalVersion = version.Get()
}

func (suite *VersionTestSuite) TearDownSuite() {
	version.Set(suite.originalVersion)
}

func (suite *VersionTestSuite) TestIsControllerVersionStale() {
	for _, tc := range []struct {
		name    string
		current string
		stamped string
		stale   bool
	}{
		{name: "equal semver", current: "1.16.5", stamped: "1.16.5", stale: false},
		{name: "older semver", current: "1.16.5", stamped: "1.16.4", stale: true},
		{name: "older minor", current: "1.16.5", stamped: "1.15.27", stale: true},
		{name: "newer semver", current: "1.16.5", stamped: "1.16.6", stale: false},
		{name: "equal non-semver", current: "latest", stamped: "latest", stale: false},
		{name: "different non-semver falls back to inequality", current: "latest", stamped: "1.16.4", stale: true},
		{name: "empty stamp", current: "1.16.5", stamped: "", stale: true},
	} {
		suite.Run(tc.name, func() {
			version.Set(&version.Info{Label: tc.current})
			suite.Equal(tc.stale, IsControllerVersionStale(tc.stamped))
		})
	}
}

func TestVersionTestSuite(t *testing.T) {
	suite.Run(t, new(VersionTestSuite))
}
