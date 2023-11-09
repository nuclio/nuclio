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

package kubectlclient

import "fmt"

type RunKubectlCommandMode string

const (
	RunKubectlCommandDirect   RunKubectlCommandMode = "direct"
	RunKubectlCommandMinikube RunKubectlCommandMode = "minikube"
)

type CommandExecutor interface {
	GetRunCommand() string
	GetKind() RunKubectlCommandMode
}

type minikubeRunOptions struct {
	Profile string
}

func (m *minikubeRunOptions) GetRunCommand() string {
	return fmt.Sprintf("minikube --profile %s kubectl --", m.Profile)
}

func (m *minikubeRunOptions) GetKind() RunKubectlCommandMode {
	return RunKubectlCommandMinikube
}

type directRunOptions struct {
}

func (d *directRunOptions) GetRunCommand() string {
	return "kubectl"
}

func (d *directRunOptions) GetKind() RunKubectlCommandMode {
	return RunKubectlCommandDirect
}
