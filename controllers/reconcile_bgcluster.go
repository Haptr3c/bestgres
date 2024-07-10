package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	bestgresv1 "bestgres/api/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	workerListAnnotation = "bgshardedcluster.bestgres.io/workers"
	initializedAnnotation = "bgcluster.bestgres.io/initialized"
)

func (r *BGShardedClusterReconciler) reconcileCoordinatorBGCluster(ctx context.Context, bgShardedCluster *bestgresv1.BGShardedCluster) error {
	coordinatorName := fmt.Sprintf("%s-coordinator", bgShardedCluster.Name)
	err := r.reconcileBGCluster(ctx, bgShardedCluster, coordinatorName, bgShardedCluster.Spec.Coordinator, true)
	if err != nil {
		return err
	}

	// Update worker list annotation on coordinator
	return r.updateWorkerListAnnotation(ctx, bgShardedCluster)
}

func (r *BGShardedClusterReconciler) reconcileWorkerBGClusters(ctx context.Context, bgShardedCluster *bestgresv1.BGShardedCluster) ([]string, error) {
	workerClusters := []string{}
	for i := 0; i < int(bgShardedCluster.Spec.Shards); i++ {
		workerName := fmt.Sprintf("%s-worker-%d", bgShardedCluster.Name, i)
		if err := r.reconcileBGCluster(ctx, bgShardedCluster, workerName, bgShardedCluster.Spec.Workers, false); err != nil {
			return nil, err
		}
		workerClusters = append(workerClusters, workerName)
	}
	
	// Update coordinator annotations based on worker statuses
	if err := r.updateCoordinatorAnnotations(ctx, bgShardedCluster); err != nil {
		return nil, err
	}

	return workerClusters, nil
}

func (r *BGShardedClusterReconciler) reconcileBGCluster(ctx context.Context, bgShardedCluster *bestgresv1.BGShardedCluster, name string, spec bestgresv1.BGClusterSpec, isCoordinator bool) error {
	bgCluster := &bestgresv1.BGCluster{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: bgShardedCluster.Namespace}, bgCluster)

	if err != nil {
		if errors.IsNotFound(err) {
			return r.createBGCluster(ctx, bgShardedCluster, name, spec, isCoordinator)
		}
		return err
	}
	return r.updateBGCluster(ctx, bgCluster, spec)
}

func (r *BGShardedClusterReconciler) createBGCluster(ctx context.Context, bgShardedCluster *bestgresv1.BGShardedCluster, name string, spec bestgresv1.BGClusterSpec, isCoordinator bool) error {
	logger := log.FromContext(ctx)

	bgCluster := &bestgresv1.BGCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: bgShardedCluster.Namespace,
			Labels: map[string]string{
				"bgcluster.bestgres.io/part-of": bgShardedCluster.Name,
				"bgcluster.bestgres.io/role":    map[bool]string{true: "coordinator", false: "worker"}[isCoordinator],
			},
		},
		Spec: spec,
	}

	if err := ctrl.SetControllerReference(bgShardedCluster, bgCluster, r.Scheme); err != nil {
		return err
	}

	logger.Info("Creating a new BGCluster", "BGCluster.Namespace", bgCluster.Namespace, "BGCluster.Name", bgCluster.Name)
	return r.Create(ctx, bgCluster)
}

func (r *BGShardedClusterReconciler) updateBGCluster(ctx context.Context, bgCluster *bestgresv1.BGCluster, spec bestgresv1.BGClusterSpec) error {
	logger := log.FromContext(ctx)

	if !reflect.DeepEqual(bgCluster.Spec, spec) {
		bgCluster.Spec = spec
		logger.Info("Updating BGCluster", "BGCluster.Namespace", bgCluster.Namespace, "BGCluster.Name", bgCluster.Name)
		return r.Update(ctx, bgCluster)
	}

	return nil
}

func (r *BGShardedClusterReconciler) updateWorkerListAnnotation(ctx context.Context, bgShardedCluster *bestgresv1.BGShardedCluster) error {
	coordinatorName := fmt.Sprintf("%s-coordinator", bgShardedCluster.Name)
	coordinator := &bestgresv1.BGCluster{}
	err := r.Get(ctx, types.NamespacedName{Name: coordinatorName, Namespace: bgShardedCluster.Namespace}, coordinator)
	if err != nil {
		return err
	}

	workerList := make([]string, bgShardedCluster.Spec.Shards)
	for i := 0; i < int(bgShardedCluster.Spec.Shards); i++ {
		workerList[i] = fmt.Sprintf("%s-worker-%d", bgShardedCluster.Name, i)
	}

	// Marshal the worker list to JSON
	workerListJSON, err := json.Marshal(workerList)
	if err != nil {
		return fmt.Errorf("failed to marshal worker list to JSON: %w", err)
	}

	if coordinator.Annotations == nil {
		coordinator.Annotations = make(map[string]string)
	}
	coordinator.Annotations[workerListAnnotation] = string(workerListJSON)

	return r.Update(ctx, coordinator)
}

func (r *BGShardedClusterReconciler) updateCoordinatorAnnotations(ctx context.Context, bgShardedCluster *bestgresv1.BGShardedCluster) error {
	coordinatorName := fmt.Sprintf("%s-coordinator", bgShardedCluster.Name)
	coordinator := &bestgresv1.BGCluster{}
	err := r.Get(ctx, types.NamespacedName{Name: coordinatorName, Namespace: bgShardedCluster.Namespace}, coordinator)
	if err != nil {
		return err
	}

	updated := false
	if coordinator.Annotations == nil {
		coordinator.Annotations = make(map[string]string)
	}

	for i := 0; i < int(bgShardedCluster.Spec.Shards); i++ {
		workerName := fmt.Sprintf("%s-worker-%d", bgShardedCluster.Name, i)
		worker := &bestgresv1.BGCluster{}
		err := r.Get(ctx, types.NamespacedName{Name: workerName, Namespace: bgShardedCluster.Namespace}, worker)
		if err != nil {
			return err
		}

		if initStatus, exists := worker.Annotations[initializedAnnotation]; exists {
			coordinatorAnnotationKey := fmt.Sprintf("bgshardedcluster.bestgres.io/%s-initialized", workerName)
			if coordinator.Annotations[coordinatorAnnotationKey] != initStatus {
				coordinator.Annotations[coordinatorAnnotationKey] = initStatus
				updated = true
			}
		}
	}

	if updated {
		return r.Update(ctx, coordinator)
	}

	return nil
}