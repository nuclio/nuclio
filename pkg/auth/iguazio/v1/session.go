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

package v1

import (
	"encoding/base64"
	"fmt"

	"github.com/nuclio/nuclio/pkg/auth/iguazio"
)

type Session struct {
	*iguazio.AbstractSession
	SessionKey string
	UserID     string
}

func NewSession(username, sessionKey, userID string, groupIDs []string) *Session {
	return &Session{AbstractSession: &iguazio.AbstractSession{
		Username: username,
		GroupIDs: groupIDs,
	}, SessionKey: sessionKey, UserID: userID}
}

func (s *Session) CompileAuthorizationBasic() string {
	data := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", s.Username, s.SessionKey)))
	return fmt.Sprintf("Basic: %s", data)
}

func (s *Session) GetUserID() string {
	return s.UserID
}

func (s *Session) GetPassword() string {
	return s.SessionKey
}
