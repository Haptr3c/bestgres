#!/bin/bash

set -xeo pipefail

kubectl delete -f examples/bgshardedcluster.yaml || true
helm uninstall bestgres-operator || true
kubectl delete pvc pgdata-test-bgcluster-0 || true
kubectl delete pvc pgdata-test-bgcluster-1 || true
kubectl delete pvc controller-test-bgcluster-0 || true
kubectl delete pvc controller-test-bgcluster-1 || true
kubectl delete pvc pgdata-bgshardedcluster-coordinator-0 || true
kubectl delete pvc pgdata-bgshardedcluster-worker-0-0 || true
kubectl delete pvc pgdata-bgshardedcluster-worker-1-0 || true
kubectl delete pvc controller-bgshardedcluster-coordinator-0 || true
kubectl delete pvc controller-bgshardedcluster-worker-0-0 || true
kubectl delete pvc controller-bgshardedcluster-worker-1-0 || true
kubectl delete cm bgcluster-0-leader || true
kubectl delete cm bgcluster-1-leader || true
kubectl delete cm bgshardedcluster-worker-0-leader || true
kubectl delete cm bgshardedcluster-worker-1-leader || true
kubectl delete cm bgshardedcluster-coordinator-leader || true

kubectl delete crd bgclusters.bestgres.io || true
kubectl delete crd bgshardedclusters.bestgres.io || true
kubectl delete crd bgdbops.bestgres.io || true

make

helm upgrade --install bestgres-operator deploy/helm/bestgres-operator/.
kubectl apply -f examples/bgshardedcluster.yaml

sleep 10

kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c 'SELECT * from pg_dist_node;'