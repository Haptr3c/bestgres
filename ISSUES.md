# Issues

1. [ ] Configmaps don't always get deleted for some reason? need to verify that they have the right ownership. Might be a race condition issue with testing script.
2. [ ] Coordinator gets stuck with no leader configmap sometimes, need to investigate why this happens.
3. [ ] Replicas for shardedclusters aren't working, need to make specific bootstrap codepaths for that situation, right now they're just trying to do the default for sharded coord/worker bootstrapping.
