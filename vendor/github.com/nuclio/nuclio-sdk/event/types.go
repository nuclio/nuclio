package event

import (
	"github.com/satori/go.uuid"
)

type ID *uuid.UUID

func NewID() ID {
	// create a unique request ID
	id := uuid.NewV4()
	return ID(&id)
}
