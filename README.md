
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
- [x] add support for bgshardedclusters
  - [x] get citus working on shards
  - [ ] add signaling for coordinator/workers
- [ ] cleanup icky controller/bootstrap code
- [ ] get sgdbops communications working
- [ ] polish user experience
  - [ ] better error messages
  - [ ] cleaner/better CRD structure
- [ ] add support for pgbackups
- [ ] add support for pgrestores
- [ ] add support for arbitrary pg extensions via oci image
- [ ] add support for pgupgrades

Prompt:
```
You are creating "Bestgres" the best postgres kubernetes operator

The core principles of the operator are:
- Minimal dependencies for security and simplicity
- Uses vanila or user-supplied images and maintains full compatibility with upstream patroni
- No: 
  - Webhooks
  - pods/exec
  - cluster-scoped resources
  - cluster-scoped permissions
  - running as root
  - use of endpoints

```