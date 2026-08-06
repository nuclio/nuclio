//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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

package git

import (
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/suite"
)

type AttributesTestSuite struct {
	suite.Suite
}

// TestDecodeCodeEntryAttributesWithMTLSFields mirrors how builder.go's parseGitAttributes
// decodes spec.build.codeEntryAttributes (a map[string]interface{} straight out of the
// function config) into an Attributes struct, to ensure the expected keys (clientCert,
// clientKey, caBundle) decode correctly with mapstructure's default matching.
func (suite *AttributesTestSuite) TestDecodeCodeEntryAttributesWithMTLSFields() {
	codeEntryAttributes := map[string]interface{}{
		"branch":     "main",
		"username":   "myuser",
		"password":   "mypassword",
		"clientCert": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
		"clientKey":  "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----",
		"caBundle":   "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
	}

	var attributes Attributes
	err := mapstructure.Decode(codeEntryAttributes, &attributes)
	suite.Require().NoError(err)

	suite.Require().Equal("main", attributes.Branch)
	suite.Require().Equal("myuser", attributes.Username)
	suite.Require().Equal("mypassword", attributes.Password)
	suite.Require().Equal("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----", attributes.ClientCert)
	suite.Require().Equal("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----", attributes.ClientKey)
	suite.Require().Equal("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----", attributes.CABundle)
}

func (suite *AttributesTestSuite) TestDecodeCodeEntryAttributesWithoutMTLSFieldsLeavesThemEmpty() {
	codeEntryAttributes := map[string]interface{}{
		"branch": "main",
	}

	var attributes Attributes
	err := mapstructure.Decode(codeEntryAttributes, &attributes)
	suite.Require().NoError(err)

	suite.Require().Empty(attributes.ClientCert)
	suite.Require().Empty(attributes.ClientKey)
	suite.Require().Empty(attributes.CABundle)
}

// TestBuildCloneOptionsSetsMTLSFields guards against a future refactor of buildCloneOptions
// (e.g. reordering fields, or swapping ClientCert/ClientKey) silently breaking mutual TLS --
// git.PlainClone itself isn't mockable, so this is the only seam that can catch that class of
// regression without a real git server.
func (suite *AttributesTestSuite) TestBuildCloneOptionsSetsMTLSFields() {
	attributes := &Attributes{
		ClientCert: "cert-pem",
		ClientKey:  "key-pem",
		CABundle:   "ca-pem",
	}

	options := buildCloneOptions("https://example.com/repo.git", "refs/heads/main", nil, attributes)

	suite.Require().Equal("https://example.com/repo.git", options.URL)
	suite.Require().Equal([]byte("cert-pem"), options.ClientCert)
	suite.Require().Equal([]byte("key-pem"), options.ClientKey)
	suite.Require().Equal([]byte("ca-pem"), options.CABundle)
}

func (suite *AttributesTestSuite) TestBuildCloneOptionsLeavesMTLSFieldsEmptyWhenUnset() {
	options := buildCloneOptions("https://example.com/repo.git", "refs/heads/main", nil, &Attributes{})

	suite.Require().Empty(options.ClientCert)
	suite.Require().Empty(options.ClientKey)
	suite.Require().Empty(options.CABundle)
}

func TestAttributesTestSuite(t *testing.T) {
	suite.Run(t, new(AttributesTestSuite))
}
