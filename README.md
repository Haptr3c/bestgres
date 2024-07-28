# Bestgres

**The Best Postgres Operator for Kubernetes**

## Getting Started

- Install [go](https://code.visualstudio.com/docs/languages/go)
- Install [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) and [helm](https://helm.sh/docs/intro/install/)
- Install [docker desktop](https://docs.docker.com/desktop/install/mac-install/) and [enable kubernetes](https://docs.docker.com/desktop/kubernetes/)
- Clone the spilo repo and build postgres images for use with the bestgres operator
  ```bash
  git clone https://github.com/zalando/spilo.git
  cd spilo/postgres-appliance
  docker build -t spilo:16 .
  ```
- use the makefile to build the operator image and CRDs
  ```bash
  make
  ```
- use helm to deploy the operator
  ```bash
  helm upgrade --install bestgres-operator deploy/helm/bestgres-operator/.
  ```
- deploy a test postgres cluster using one of the example manifests
  ```bash
  kubectl apply -f examples/bgcluster.yaml
  ```
- refer to the [test.sh](./test.sh) file for reference on running SQL commands against postgres clusters

## Principles

- Minimal dependencies for security and simplicity
- Uses vanila or user-supplied images and maintains full compatibility with upstream patroni
- No:
  - Webhooks
  - pods/exec
  - cluster-scoped resources
  - cluster-scoped permissions
  - running as root
  - use of endpoints
- Airgapped-ready

## TODO

- [x] implement initContainer/controller pattern
- [x] add support for db replicas
- [x] remove and test without endpoints perms
- [x] cleanup icky controller/bootstrap code
- [x] add support for bgshardedclusters
  - [x] get citus working on shards
  - [x] add signaling for coordinator/workers
    - [x] bgshardedcluster lists all shards via annotation on coordinator
    - [x] operator reconciliation reads and updates shard list
    - [x] workers update shard annotations as they come online
    - [x] coordinator adds workers to citus as they report via annotations
- [x] setup a proper logger for the controller
- [x] get bgdbops communications working
- [x] fix replicas
- [x] add db restart bgdbops
- [x] add db restart bgshardeddbops
- [ ] add spindown safety
  - [ ] add finalizers
  - [ ] add clean db shutdown handling
  - [ ] handle main process better (stop controller when main crashes)
- [ ] polish user experience
  - [ ] add cr status
  - [ ] better error messages
  - [ ] cleaner/better CRD structure
- [ ] test/handle adding/removing shards
- [ ] add auto-rebalance on shard addition (with option to disable)
- [ ] check if possible to leverage patroni's native citus support (may not fit reqs) [reference](https://patroni.readthedocs.io/en/latest/ENVIRONMENT.html#citus)
- [ ] add support for pgbackups
- [ ] add support for pgrestores
- [ ] add support for pgupgrades
- [ ] add controller handling for replicas of sharded clusters
- [ ] (maybe) add support for arbitrary pg extensions via oci image

Prompt:

> You are creating "Bestgres" the best postgres kubernetes operator
>
> The core principles of the operator are:
>
> - Minimal dependencies for security and simplicity
> - Uses vanila or user-supplied images and maintains full compatibility with upstream patroni
> - No:
>   - Webhooks
>   - pods/exec
>   - cluster-scoped resources
>   - cluster-scoped permissions
>   - running as root

<!-- >   - use of endpoints -->


## Referernce

- [patroni env var settings](https://patroni.readthedocs.io/en/latest/ENVIRONMENT.html#kubernetes)
- [spilo operator options](https://postgres-operator.readthedocs.io/en/latest/reference/operator_parameters/)
