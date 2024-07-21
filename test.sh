#!/bin/bash

set -xeo pipefail

kubectl delete -f examples/bgdbops.yaml || true
kubectl delete -f examples/bgshardeddbops.yaml || true
kubectl delete -f examples/bgshardedcluster.yaml --wait || true
kubectl delete -f examples/bgshardedcluster-replicas.yaml --wait || true
kubectl delete -f examples/bgcluster.yaml --wait || true
sleep 5
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
kubectl delete pvc pgdata-bgshardedcluster-coordinator-1 || true
kubectl delete pvc pgdata-bgshardedcluster-worker-0-0 || true
kubectl delete pvc pgdata-bgshardedcluster-worker-0-1 || true
kubectl delete pvc pgdata-bgshardedcluster-worker-1-0 || true
kubectl delete pvc controller-bgshardedcluster-coordinator-0 || true
kubectl delete pvc controller-bgshardedcluster-coordinator-1 || true
kubectl delete pvc controller-bgshardedcluster-worker-0-0 || true
kubectl delete pvc controller-bgshardedcluster-worker-0-1 || true
kubectl delete pvc controller-bgshardedcluster-worker-1-0 || true
kubectl delete cm bgshardedcluster-worker-0-leader || true
kubectl delete cm bgshardedcluster-worker-0-config || true
kubectl delete cm bgshardedcluster-worker-1-leader || true
kubectl delete cm bgshardedcluster-worker-1-config || true
kubectl delete cm bgshardedcluster-coordinator-leader || true
kubectl delete pvc controller-bgshardedcluster-repl-coordinator-0 || true
kubectl delete pvc controller-bgshardedcluster-repl-coordinator-1 || true
kubectl delete pvc controller-bgshardedcluster-repl-worker-0-0 || true
kubectl delete pvc controller-bgshardedcluster-repl-worker-0-1 || true
kubectl delete pvc controller-bgshardedcluster-repl-worker-1-0 || true
kubectl delete pvc controller-bgshardedcluster-repl-worker-1-1 || true
kubectl delete pvc pgdata-bgshardedcluster-repl-coordinator-0 || true
kubectl delete pvc pgdata-bgshardedcluster-repl-coordinator-1 || true
kubectl delete pvc pgdata-bgshardedcluster-repl-worker-0-0 || true
kubectl delete pvc pgdata-bgshardedcluster-repl-worker-0-1 || true
kubectl delete pvc pgdata-bgshardedcluster-repl-worker-1-0 || true
kubectl delete pvc pgdata-bgshardedcluster-repl-worker-1-1 || true

# delete crds
kubectl delete crd bgclusters.bestgres.io || true
kubectl delete crd bgshardedclusters.bestgres.io || true
kubectl delete crd bgdbops.bestgres.io || true
kubectl delete crd bgshardeddbops.bestgres.io || true

make

helm upgrade --install bestgres-operator deploy/helm/bestgres-operator/.
# kubectl apply -f examples/bgcluster.yaml
kubectl apply -f examples/bgshardedcluster.yaml

# watch 'kubectl get bgcluster -o=json | jq ".items[].metadata.annotations"'

sleep 30
# kubectl exec -it bgcluster-0 -- pg_isready

# kubectl exec -it bgcluster-0 -- patronictl list
# kubectl exec -it bgcluster-0 -- psql -U postgres -c 'SELECT * FROM pg_stat_replication;'
# kubectl exec -it bgcluster-1 -- psql -U postgres -c 'SELECT * FROM pg_stat_wal_receiver;'

kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c 'SELECT * from pg_dist_node;'
# kubectl exec -it bgshardedcluster-repl-coordinator-0 -- psql -U postgres -c 'SELECT * from pg_dist_node;'
# kubectl exec -it bgshardedcluster-repl-coordinator-0 -- patronictl list
# kubectl exec -it bgcluster-0 -- psql -U postgres -c 'CREATE TABLE test_table (id SERIAL PRIMARY KEY, name VARCHAR(100) NOT NULL, age INT NOT NULL);'
# kubectl exec -it bgcluster-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Alice', 30);"
# kubectl exec -it bgcluster-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Bob', 25);"
# kubectl exec -it bgcluster-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Charlie', 35);"
# kubectl exec -it bgcluster-0 -- psql -U postgres -c "SELECT * FROM test_table;"
# sleep 30
# kubectl exec -it bgcluster-1 -- psql -U postgres -c "SELECT * FROM test_table;"

# kubectl apply -f examples/bgdbops.yaml

kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c 'CREATE TABLE test_table (id SERIAL PRIMARY KEY, name VARCHAR(100) NOT NULL, age INT NOT NULL);'
kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Alice', 30);"
kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Bob', 25);"
kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Charlie', 35);"
kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Daniel', 29);"
kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Eve', 27);"
kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Frank', 33);"
kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Grace', 31);"
kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Hank', 28);"
kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Ivy', 34);"
kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Jack', 32);"
kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "INSERT INTO test_table (name, age) VALUES ('Katie', 26);"

kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "SELECT * FROM test_table;"

kubectl exec -it bgshardedcluster-coordinator-0 -- psql -U postgres -c "SELECT create_distributed_table('test_table', 'id');"

kubectl exec -it bgshardedcluster-worker-0-0 -- psql -U postgres -c "SELECT * FROM test_table;"

# kubectl apply -f examples/bgshardeddbops.yaml
