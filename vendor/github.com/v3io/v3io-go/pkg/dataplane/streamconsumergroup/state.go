package streamconsumergroup

import (
	"encoding/json"
)

type State struct {
	SchemasVersion string          `json:"schema_version"`
	SessionStates  []*SessionState `json:"session_states"`
}

func newState() (*State, error) {
	return &State{
		SchemasVersion: "0.0.1",
		SessionStates:  []*SessionState{},
	}, nil
}

func (s *State) String() string {
	marshalledState, err := json.Marshal(s)
	if err != nil {
		return err.Error()
	}

	return string(marshalledState)
}

func (s *State) deepCopy() *State {
	stateCopy := State{}
	stateCopy.SchemasVersion = s.SchemasVersion
	for _, stateSession := range s.SessionStates {
		stateSessionCopy := stateSession
		stateCopy.SessionStates = append(stateCopy.SessionStates, stateSessionCopy)
	}

	return &stateCopy
}

func (s *State) findSessionStateByMemberID(memberID string) *SessionState {
	for _, sessionState := range s.SessionStates {
		if sessionState.MemberID == memberID {
			return sessionState
		}
	}

	return nil
}
