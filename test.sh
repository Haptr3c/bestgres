#!/bin/zsh

set -xeo pipefail

kubectl delete -f examples/bgcluster.yaml || true
helm uninstall bestgres-operator || true
kubectl delete pvc pgdata-example-bgcluster-0 || true
kubectl delete pvc controller-example-bgcluster-0 || true

kubectl delete crd bgclusters.bestgres.io || true
kubectl delete crd bgdbops.bestgres.io || true

make

helm upgrade --install bestgres-operator deploy/helm/bestgres-operator/.
kubectl apply -f examples/bgcluster.yaml
