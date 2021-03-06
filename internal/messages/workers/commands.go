package workers

import (
	"github.com/juan-lee/carp/api/v1alpha1"
	"github.com/juan-lee/carp/internal/messages"
)

type PutCluster struct {
	messages.Command
	Spec v1alpha1.ManagedClusterSpec
}

type DeleteCluster struct {
	messages.Command
	Id string
}
