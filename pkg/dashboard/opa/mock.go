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

package opa

import (
	"github.com/nuclio/logger"
)

type MockClient struct {
	logger  logger.Logger
	requests []PermissionRequestInput
	answer bool
}

func NewMockClient(parentLogger logger.Logger) *MockClient {
	newClient := MockClient{
		logger:  parentLogger.GetChild("mock-opa"),
		answer: true,
	}
	return &newClient
}

func (m *MockClient) QueryPermissions(resource string, action Action, ids []string) (bool, error) {
	m.logger.DebugWith("Checking permissions in OPA (mock)",
		"resource", resource,
		"action", action,
		"ids", ids)

	m.requests = append(m.requests, PermissionRequestInput{
		Resource: resource,
		Action:   string(action),
		Ids:      ids,
	})

	return m.answer, nil
}

func (m *MockClient) SetAnswer(answer bool) {
	m.answer = answer
}

func (m *MockClient) GetRequests() []PermissionRequestInput {
	return m.requests
}

func (m *MockClient) ClearRequests() {
	m.requests = []PermissionRequestInput{}
}
