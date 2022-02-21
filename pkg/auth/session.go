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
