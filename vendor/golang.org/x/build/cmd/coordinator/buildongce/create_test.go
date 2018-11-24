// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main // import "golang.org/x/build/cmd/coordinator/buildongce"

import (
	"bytes"
	"testing"
	"text/template"

	"golang.org/x/build/buildenv"
)

func TestDeploymentManagerManifest(t *testing.T) {
	tests := []struct {
		env      *buildenv.Environment
		kube     *buildenv.KubeConfig
		expected string
	}{
		{buildenv.Staging, &buildenv.Staging.KubeBuild, `
resources:
- name: "buildlets"
  type: container.v1.cluster
  properties:
    zone: "us-central1-f"
    cluster:
      initial_node_count: 1
      network: "default"
      logging_service: "logging.googleapis.com"
      monitoring_service: "none"
      node_config:
        machine_type: "n1-standard-4"
        oauth_scopes:
          - "https://www.googleapis.com/auth/cloud-platform"
          - "https://www.googleapis.com/auth/userinfo.email"
      master_auth:
        username: "admin"
        password: ""`},
	}
	for _, test := range tests {
		tpl, err := template.New("kube").Parse(kubeConfig)
		if err != nil {
			t.Errorf("could not parse Deployment Manager template: %v", err)
		}

		var result bytes.Buffer
		err = tpl.Execute(&result, deploymentTemplateData{Env: test.env, Kube: test.kube})
		if err != nil {
			t.Errorf("could not execute Deployment Manager template: %v", err)
		}
		if result.String() != test.expected {
			t.Errorf("Rendered template did not match. Rendered: %v\n\n\nExpected: %v\n", result.String(), test.expected)
		}
	}
}
