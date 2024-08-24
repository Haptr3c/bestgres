package controllers

import (
	"context"
	"encoding/json"

	bestgresv1 "bestgres/api/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	bgDbOpsPendingAnnotation    = "bgdbops.bestgres.io/pending"
	bgDbOpsCompletedAnnotation  = "bgdbops.bestgres.io/completed"
	bgDbOpsInProgressAnnotation = "bgdbops.bestgres.io/in-progress"
	bgDbOpsSpecAnnotation       = "bgdbops.bestgres.io/spec"
	bgDbOpsOpAnnotation         = "bgdbops.bestgres.io/op"
)

func (r *BGDbOpsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the BGDbOps instance
	bgDbOps := &bestgresv1.BGDbOps{}
	err := r.Get(ctx, req.NamespacedName, bgDbOps)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if the BGDbOps is already completed
	if bgDbOps.Annotations[bgDbOpsCompletedAnnotation] == "true" {
		logger.Info("BGDbOps completed")
		return ctrl.Result{}, nil
	}

	// Fetch the target BGCluster
	bgCluster := &bestgresv1.BGCluster{}
	err = r.Get(ctx, types.NamespacedName{Name: bgDbOps.Spec.BGCluster, Namespace: bgDbOps.Namespace}, bgCluster)
	if err != nil {
		logger.Error(err, "Unable to fetch BGCluster")
		return ctrl.Result{}, err
	}

	// Check if all pods that are members of the BGCluster have completed the operation
	podList := &corev1.PodList{}
	clusterLabels := make(map[string]string)
	clusterLabels["cluster-name"] = bgCluster.Name
	if err := r.List(ctx, podList, client.InNamespace(bgCluster.Namespace), client.MatchingLabels(clusterLabels)); err != nil {
		logger.Error(err, "Unable to list pods")
		return ctrl.Result{}, err
	}

	allCompleted := true
	anyInProgress := false
	for _, pod := range podList.Items {
		if pod.Annotations[bgDbOpsCompletedAnnotation] != "true" {
			allCompleted = false
			if pod.Annotations[bgDbOpsInProgressAnnotation] == bgDbOps.Name {
				anyInProgress = true
			}
		}
	}

	if allCompleted {
		// Remove operation annotations and set pending to false
		delete(bgCluster.Annotations, bgDbOpsOpAnnotation)
		delete(bgCluster.Annotations, bgDbOpsSpecAnnotation)
		delete(bgCluster.Annotations, bgDbOpsPendingAnnotation)
		delete(bgCluster.Annotations, bgDbOpsInProgressAnnotation)

		if err := r.Update(ctx, bgCluster); err != nil {
			logger.Error(err, "Unable to update BGCluster annotations after completion")
			return ctrl.Result{}, err
		}
		// Mark the bgdbops as completed
		bgDbOps.Annotations[bgDbOpsCompletedAnnotation] = "true"
		if err := r.Update(ctx, bgDbOps); err != nil {
			logger.Error(err, "Unable to update BGDbOps annotations after completion")
			return ctrl.Result{}, err
		}
		logger.Info("All pods completed the operation, BGDbOps marked as completed")
		return ctrl.Result{}, nil
	}

	// If any pod is still in progress, we don't need to start a new operation
	if anyInProgress {
		logger.Info("Operation still in progress on some pods")
		return ctrl.Result{}, nil
	}

	// If we reach here, it means not all pods have completed, and no pod is currently in progress
	// So we need to start/continue the operation

	// Set annotations on BGCluster
	if bgCluster.Annotations == nil {
		bgCluster.Annotations = make(map[string]string)
	}
	logger.Info("BGDBOps continuing or starting on BGCluster", "bgCluster", bgCluster.Name)
	bgCluster.Annotations[bgDbOpsPendingAnnotation] = "true"
	bgCluster.Annotations[bgDbOpsOpAnnotation] = string(bgDbOps.Spec.Op)
	bgCluster.Annotations[bgDbOpsInProgressAnnotation] = bgDbOps.Name

	// Marshal other options to JSON
	specJSON, err := json.Marshal(bgDbOps.Spec)
	if err != nil {
		logger.Error(err, "Unable to marshal BGDbOps spec")
		return ctrl.Result{}, err
	}
	bgCluster.Annotations[bgDbOpsSpecAnnotation] = string(specJSON)

	// Update BGCluster
	if err := r.Update(ctx, bgCluster); err != nil {
		logger.Error(err, "Unable to update BGCluster annotations")
		return ctrl.Result{}, err
	}

	logger.Info("BGCluster annotations updated to continue/start operation")
	return ctrl.Result{}, nil
}