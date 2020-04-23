package workers

import (
	"github.com/juan-lee/carp/api/v1alpha1"
	"github.com/juan-lee/carp/messages"
)

type(
	ClusterStatusChanged struct {
		messages.Event
		Status v1alpha1.ManagedClusterStatus
	}
)
