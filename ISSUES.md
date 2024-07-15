# Issues

1. [ ] ~~Controller needs to update context every time it wants to patch/update, otherwise may get:~~
    - ~~This should be fixed now I think? Observe in case of continued errant behavior.~~
    - Still seeing it happen a bit, need to double check all the places where we're updating the object and make sure it's always pulling fresh context.
    `Failed to update BGCluster: Operation cannot be fulfilled on bgclusters.bestgres.io "bgshardedcluster-coordinator": the object has been modified; please apply your changes to the latest version and try again`
2. [x] ~~Master/replica broken, might be due to removal of endpoints?~~
    - ~~seems networking related, can't hit the master svc from the replica with creds~~
    - ~~doesn't seem to be endpoint related added back all perms but still broken~~
    - ~~maybe [this](https://github.com/Haptr3c/bestgres/compare/29e46a0a89789ff591df3327fd3e1f37e2ff52f5...main#diff-3dc09f1f3c24c29007908270118355059cc4ae947a1c8a50ea4e0cefb8f68d76L166-R124)?~~
    - ~~check replication credentials~~
    - ~~take out BGMON_LISTEN_IP (probably not needed)~~
3. [x] ~~Replicas fight over the initialized: true annotation~~
    - ~~switched to operator handling setting "initialized" rather than controller~~
4. [ ] Configmaps don't always get deleted for some reason? need to verify that they have the right ownership. Might be a race condition issue with testing script.
5. [ ] Coordinator gets stuck with no leader configmap sometimes, need to investigate why this happens.
6. [ ] Same as issue #1 but with the coordinator as well.
    ```
2024-07-14T01:58:10Z    ERROR    Failed to update BGCluster status    {"controller": "bgcluster", "controllerGroup": "bestgres.io", "controllerKind": "BGCluster", "BGCluster": {"name":"bgcluster","namespace":"default"}, "namespace": "default", "name": "bgcluster", "reconcileID": "675b9fea-ea79-4b09-b565-5a69fadd5200", "error": "Operation cannot be fulfilled on bgclusters.bestgres.io \"bgcluster\": the object has been modified; please apply your changes to the latest version and try again"}
bestgres/controllers.(*BGClusterReconciler).Reconcile
    /app/controllers/bgcluster_controller.go:85
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).Reconcile
    /go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.4/pkg/internal/controller/controller.go:114
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).reconcileHandler
    /go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.4/pkg/internal/controller/controller.go:311
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).processNextWorkItem
    /go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.4/pkg/internal/controller/controller.go:261
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).Start.func2.2
    /go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.4/pkg/internal/controller/controller.go:222
2024-07-14T01:58:10Z    ERROR    Reconciler error    {"controller": "bgcluster", "controllerGroup": "bestgres.io", "controllerKind": "BGCluster", "BGCluster": {"name":"bgcluster","namespace":"default"}, "namespace": "default", "name": "bgcluster", "reconcileID": "675b9fea-ea79-4b09-b565-5a69fadd5200", "error": "Operation cannot be fulfilled on bgclusters.bestgres.io \"bgcluster\": the object has been modified; please apply your changes to the latest version and try again"}
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).reconcileHandler
    /go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.4/pkg/internal/controller/controller.go:324
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).processNextWorkItem
    /go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.4/pkg/internal/controller/controller.go:261
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).Start.func2.2
    /go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.4/pkg/internal/controller/controller.go:222
    ```
7. [ ] Replicas for shardedclusters aren't working, probalby need to make specific bootstrap codepaths for that situation, right now they're just trying to do the default for sharded coord/worker bootstrapping.