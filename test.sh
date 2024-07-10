#!/bin/bash

set -xeo pipefail

kubectl delete -f examples/bgshardedcluster.yaml --wait || true
kubectl delete -f examples/bgcluster.yaml --wait || true
helm uninstall bestgres-operator || true

# delete bgcluster stuff
kubectl delete pvc controller-bgcluster-0 || true
kubectl delete pvc controller-bgcluster-1 || true
kubectl delete pvc pgdata-bgcluster-0 || true
kubectl delete pvc pgdata-bgcluster-1 || true
kubectl delete cm bgcluster-0-leader || true
kubectl delete cm bgcluster-1-leader || true

# delete bgshardedcluster stuff
kubectl delete pvc pgdata-bgshardedcluster-coordinator-0 || true
kubectl delete pvc pgdata-bgshardedcluster-worker-0-0 || true
kubectl delete pvc pgdata-bgshardedcluster-worker-1-0 || true
kubectl delete pvc controller-bgshardedcluster-coordinator-0 || true
kubectl delete pvc controller-bgshardedcluster-worker-0-0 || true
kubectl delete pvc controller-bgshardedcluster-worker-1-0 || true
kubectl delete cm bgshardedcluster-worker-0-leader || true
kubectl delete cm bgshardedcluster-worker-1-leader || true
kubectl delete cm bgshardedcluster-coordinator-leader || true

# delete crds
kubectl delete crd bgclusters.bestgres.io || true
kubectl delete crd bgshardedclusters.bestgres.io || true
kubectl delete crd bgdbops.bestgres.io || true

make

helm upgrade --install bestgres-operator deploy/helm/bestgres-operator/.
kubectl apply -f examples/bgcluster.yaml
kubectl apply -f examples/bgshardedcluster.yaml

# watch 'kubectl get bgcluster -o=json | jq ".items[].metadata.annotations"'

sleep 30
kubectl exec -it bgcluster-0 -- patronictl list
kubectl exec -it bgcluster-0 -- psql -U postgres -c 'SELECT * FROM pg_stat_replication;'
kubectl exec -it bgcluster-1 -- psql -U postgres -c 'SELECT * FROM pg_stat_wal_receiver;'

kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c 'SELECT * from pg_dist_node;'