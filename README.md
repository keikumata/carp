# carp

![CI](https://github.com/juan-lee/carp/workflows/CI/badge.svg)

**C**luster **A**PI **R**esource **P**rovider is hackathon project that demonstrates how a
distributed system for managing kubernetes clusters could work. CARP borrows distributed system
design concepts from kubernetes, but instead of a cluster being comprised of virtual machines, a
carp cluster is made up of kubernetes clusters.

## Goals

- Simple kubernetes API for managing kubernetes clusters
- Scalable

## Design

### Control Plane

#### Control Plane Role

- flux
- carp operator (control plane mode)
- capi operator
- capz operator

#### Managed Cluster API

The Managed Cluster API defines the properties of a cluster that will be managed by carp.

##### Spec

- Cluster Spec
  - kubernetes version
  - Node Count

##### Status

- Phase
- Errors
- Assigned Worker

##### Controller Responsibilities

- Schedule cluster on a healthy Worker with available capacity

#### Worker API

The Worker API defines the properties of a cluster that will host managed kubernetes control planes.

##### Spec

- Cluster Spec
  - kubernetes version
  - Node Count
- Capacity

##### Status

- Phase
- Errors
- Available Capacity

##### Controller Responsibilities

- Provision/Manage capz cluster
- Install/Update carp Worker components via flux

### Worker

#### Worker Role

- flux
- carp operator (control plane mode)
- capi operator
- capz operator

#### Managed Cluster API

The Managed Cluster API defines the properties of a cluster that will be managed by carp.

##### Spec

- Cluster Spec
  - kubernetes version
  - Node Count

##### Status

- Phase
- Errors
- Assigned Worker

##### Controller Responsibilities

- Provision/Manage hosted control plane capz cluster

### Control Plane and Worker Coordination

#### Managed Cluster Event Publisher

The managed cluster event publisher runs as part of the carp control plane and is responsible for
publishing events for when a managed cluster is created, updated, or deleted.

##### Responsibilities

- Publish an event when a cluster is created, updated, or deleted.

#### Managed Cluster Event Subscriber

The managed cluster event subscriber runs on each carp Worker listening for new clusters to be
scheduled, existing cluster updates, and deletes.

##### Responsibilities

- Listen for cluster create, update, and delete events for clusters scheduled on the Worker its
  running on and apply or delete the latest Managed Cluster API CRD.
