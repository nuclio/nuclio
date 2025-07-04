/*
Copyright 2025 The Nuclio Authors.

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

package nop

import (
	"strings"

	"github.com/nuclio/nuclio/pkg/auth/iguazio"
)

type Session struct {
}

func (s *Session) GetUsername() string {
	return ""
}

func (s *Session) CompileAuthorizationBasic() string {
	return ""
}

func (s *Session) GetUserID() string {
	return ""
}

func (s *Session) GetPassword() string {
	return ""
}

func (s *Session) GetGroupIDs() []string {
	return []string{}
}

func (s *Session) GetUserLabels() map[string]string {
	labels := make(map[string]string)
	fullUsername := s.GetUsername()
	// split email usernames to name and domain because '@' is an invalid character in kubernetes labels
	if strings.Contains(fullUsername, "@") {
		split := strings.Split(fullUsername, "@")
		labels[iguazio.IguazioUsernameLabel] = split[0]
		labels[iguazio.IguazioDomainLabel] = split[1]
	} else {
		labels[iguazio.IguazioUsernameLabel] = fullUsername
	}
	return labels
}
