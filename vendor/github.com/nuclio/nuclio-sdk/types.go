package nuclio

import (
	"github.com/satori/go.uuid"
)

// ID is event ID
type ID struct {
	*uuid.UUID
}

// NewID creates new random event ID
func NewID() ID {
	// create a unique request ID
	id := uuid.NewV4()
	return ID{&id}
}

func (id ID) String() string {
	return id.UUID.String()
}
