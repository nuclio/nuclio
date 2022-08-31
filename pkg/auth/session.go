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

package auth

import (
	"encoding/base64"
	"fmt"
)

type abstractSession struct {
	Iguazio *IguazioSession
	Nop     *NopSession
}

func (a *abstractSession) GetUsername() string {
	return ""
}

func (a *abstractSession) CompileAuthorizationBasic() string {
	return ""
}

func (a *abstractSession) GetUserID() string {
	return ""
}

func (a *abstractSession) GetPassword() string {
	return ""
}

func (a *abstractSession) GetGroupIDs() []string {
	return []string{}
}

type IguazioSession struct {
	*abstractSession
	Username   string
	SessionKey string
	UserID     string
	GroupIDs   []string
}

func (a *IguazioSession) GetUsername() string {
	return a.Username
}

func (a *IguazioSession) CompileAuthorizationBasic() string {
	data := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", a.Username, a.SessionKey)))
	return fmt.Sprintf("Basic: %s", data)
}

func (a *IguazioSession) GetUserID() string {
	return a.UserID
}

func (a *IguazioSession) GetPassword() string {
	return a.SessionKey
}

func (a *IguazioSession) GetGroupIDs() []string {
	return a.GroupIDs
}

type NopSession struct {
	*abstractSession
}
