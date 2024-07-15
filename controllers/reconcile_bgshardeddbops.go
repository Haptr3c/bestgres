// reconcile_bgshardeddbops.go

package controllers

import (
	"context"
	"fmt"

	bestgresv1 "bestgres/api/v1"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// This function is called every time a BGShardedDbOps resource is created, updated, or deleted.
// It manages the lifecycle of BGDbOps resources for each BGCluster in the sharded setup.
func (r *BGShardedDbOpsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the BGShardedDbOps instance
	// If it doesn't exist, it may have been deleted after the reconciliation request.
	// In that case, we don't need to do anything.
	bgShardedDbOps := &bestgresv1.BGShardedDbOps{}
	err := r.Get(ctx, req.NamespacedName, bgShardedDbOps)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if the BGShardedDbOps is already completed
	// If it is, we don't need to do anything further
	if bgShardedDbOps.Status.Status == "Completed" {
		logger.Info("BGShardedDbOps already completed", "BGShardedDbOps", bgShardedDbOps.Name)
		return ctrl.Result{}, nil
	}

	// Fetch the target BGShardedCluster
	// We need this to get the names of the coordinator and worker clusters
	bgShardedCluster := &bestgresv1.BGShardedCluster{}
	err = r.Get(ctx, types.NamespacedName{Name: bgShardedDbOps.Spec.BGShardedCluster, Namespace: bgShardedDbOps.Namespace}, bgShardedCluster)
	if err != nil {
		logger.Error(err, "Unable to fetch BGShardedCluster", "BGShardedCluster", bgShardedDbOps.Spec.BGShardedCluster)
		return ctrl.Result{}, err
	}

	// Create BGDbOps for the coordinator cluster
	if err := r.createBGDbOps(ctx, bgShardedDbOps, bgShardedCluster.Status.CoordinatorCluster); err != nil {
		return ctrl.Result{}, err
	}

	// Create BGDbOps for all worker clusters
	for _, workerCluster := range bgShardedCluster.Status.WorkerClusters {
		if err := r.createBGDbOps(ctx, bgShardedDbOps, workerCluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update the status of the BGShardedDbOps to "In Progress"
	// This indicates that we've successfully created BGDbOps for all clusters
	bgShardedDbOps.Status.Status = "In Progress"
	if err := r.Status().Update(ctx, bgShardedDbOps); err != nil {
		logger.Error(err, "Unable to update BGShardedDbOps status", "BGShardedDbOps", bgShardedDbOps.Name)
		return ctrl.Result{}, err
	}

	logger.Info("BGShardedDbOps reconciliation completed successfully", "BGShardedDbOps", bgShardedDbOps.Name)
	return ctrl.Result{}, nil
}

// createBGDbOps creates a BGDbOps resource for a specific BGCluster within the sharded setup
// It copies the relevant fields from the BGShardedDbOps to create a new BGDbOps
func (r *BGShardedDbOpsReconciler) createBGDbOps(ctx context.Context, bgShardedDbOps *bestgresv1.BGShardedDbOps, bgClusterName string) error {
	logger := log.FromContext(ctx)

	// Create a new BGDbOps resource
	// The name is a combination of the BGShardedDbOps name and the BGCluster name
	bgDbOps := &bestgresv1.BGDbOps{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", bgShardedDbOps.Name, bgClusterName),
			Namespace: bgShardedDbOps.Namespace,
		},
		Spec: bestgresv1.BGDbOpsSpec{
			BGCluster:  bgClusterName,
			Op:         bgShardedDbOps.Spec.BGDbOpsClusterSpec.Op,
			MaxRetries: bgShardedDbOps.Spec.BGDbOpsClusterSpec.MaxRetries,
			Benchmark:  bgShardedDbOps.Spec.BGDbOpsClusterSpec.Benchmark,
			Repack:     bgShardedDbOps.Spec.BGDbOpsClusterSpec.Repack,
			Restart:    bgShardedDbOps.Spec.BGDbOpsClusterSpec.Restart,
			Vacuum:     bgShardedDbOps.Spec.BGDbOpsClusterSpec.Vacuum,
		},
	}

	// Try to create the BGDbOps resource
	err := r.Create(ctx, bgDbOps)
	if err != nil {
		// If the error is because the resource already exists, we can ignore it
		// This could happen if the reconciliation is triggered multiple times
		if client.IgnoreAlreadyExists(err) == nil {
			logger.Info("BGDbOps already exists", "BGDbOps", bgDbOps.Name)
			return nil
		}
		// For any other error, log it and return
		logger.Error(err, "Unable to create BGDbOps", "BGDbOps", bgDbOps.Name)
		return err
	}

	logger.Info("Successfully created BGDbOps", "BGDbOps", bgDbOps.Name)
	return nil
}