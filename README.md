# Bestgres

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
- [x] add db restart bgdbops
- [ ] add db restart bgshardeddbops
- [ ] add spindown safety
  - [ ] add finalizers
  - [ ] add clean db shutdown handling
  - [ ] handle main process better (stop controller when main crashes)
- [ ] fix replicas
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
