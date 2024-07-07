#!/bin/bash

set -xeo pipefail

kubectl delete -f examples/bgcluster.yaml || true
helm uninstall bestgres-operator || true
kubectl delete pvc pgdata-test-bgcluster-0 || true
kubectl delete pvc pgdata-test-bgcluster-1 || true
kubectl delete pvc pgdata-test-bgcluster-2 || true
kubectl delete pvc controller-test-bgcluster-0 || true
kubectl delete pvc controller-test-bgcluster-1 || true
kubectl delete pvc controller-test-bgcluster-2 || true
kubectl delete cm test-bgcluster-config || true
kubectl delete cm test-bgcluster-leader || true

kubectl delete crd bgclusters.bestgres.io || true
kubectl delete crd bgdbops.bestgres.io || true

make

helm upgrade --install bestgres-operator deploy/helm/bestgres-operator/.
kubectl apply -f examples/bgcluster.yaml
