# Issues

1. [x] ~~Controller needs to update context every time it wants to patch/update, otherwise may get:~~ This should be fixed now I think? Observe in case of continued errant behavior.
    `Failed to update BGCluster: Operation cannot be fulfilled on bgclusters.bestgres.io "bgshardedcluster-coordinator": the object has been modified; please apply your changes to the latest version and try again`
2. [ ] Master/replica broken, might be due to removal of endpoints?
3. [ ] Replicas fight over the initialized: true annotation
    - when single node, controller should handle solo, when multi-node, need to use different annotations and logic
    - this needs more thought, may need to have operator handle setting "initialized" rather than controller
