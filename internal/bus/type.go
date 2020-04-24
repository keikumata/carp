package bus

import (
	"context"

	"github.com/juan-lee/carp/internal/messages/workers"
)

type Handle func(ctx context.Context, message string) error

type Listener interface {
	Listen(ctx context.Context, h Handle) error
}

type Publisher interface {
	Publish(ctx context.Context, command workers.PutCluster) error
}
