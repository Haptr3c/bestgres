
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
- [x] add support for db replicas (primary/standby)
- [ ] add support for bgshardedclusters
- [ ] remove and test without endpoints perms
- [ ] add support for pgbackups
- [ ] add support for pgrestores
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