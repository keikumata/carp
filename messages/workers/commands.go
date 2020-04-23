package commands

import (
	"github.com/juan-lee/carp/api/v1alpha1"
	"github.com/juan-lee/carp/messages/commands"
)

type(
	PutCluster struct {
		commands.Command
		v1alpha1.ManagedClusterSpec
	}

	DeleteCluster struct {
		commands.Command
	}
)