package messages

import "github.com/google/uuid"

type Command struct {
	Id            uuid.UUID `json:"id"`
	DestinationId string    `json:"destinationId"`
}

type Event struct {
	Id uuid.UUID
}
