package messages

import "github.com/google/uuid"

type Command struct {
	Id uuid.UUID
}

type Event struct {
	Id uuid.UUID
}
