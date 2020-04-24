package workers

import (
	"github.com/juan-lee/carp/api/v1alpha1"
	"github.com/juan-lee/carp/messages"
)

type (
	PutCluster struct {
		messages.Command
		Spec v1alpha1.ManagedClusterSpec
	}

	DeleteCluster struct {
		messages.Command
		Id string
	}
)
