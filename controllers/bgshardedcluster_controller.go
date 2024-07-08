package controllers

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	bestgresv1 "bestgres/api/v1"
)

// BGShardedClusterReconciler reconciles a BGShardedCluster object
type BGShardedClusterReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Namespace string
}

//+kubebuilder:rbac:groups=bestgres.io,resources=bgshardedclusters,verbs=get;list;watch;create;update;patch;delete,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgshardedclusters/status,verbs=get;update;patch,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgshardedclusters/finalizers,verbs=update,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgclusters,verbs=get;list;watch;create;update;patch;delete,namespace="{{ .Release.Namespace }}"

func (r *BGShardedClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	bgShardedCluster := &bestgresv1.BGShardedCluster{}
	err := r.Get(ctx, req.NamespacedName, bgShardedCluster)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Reconcile coordinator BGCluster
	if err := r.reconcileCoordinatorBGCluster(ctx, bgShardedCluster); err != nil {
		logger.Error(err, "Failed to reconcile coordinator BGCluster")
		return ctrl.Result{}, err
	}

	// Reconcile worker BGClusters
	workerClusters, err := r.reconcileWorkerBGClusters(ctx, bgShardedCluster)
	if err != nil {
		logger.Error(err, "Failed to reconcile worker BGClusters")
		return ctrl.Result{}, err
	}

	// Update status
	if err := r.updateStatus(ctx, bgShardedCluster, workerClusters); err != nil {
		logger.Error(err, "Failed to update BGShardedCluster status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *BGShardedClusterReconciler) updateStatus(ctx context.Context, bgShardedCluster *bestgresv1.BGShardedCluster, workerClusters []string) error {
	coordinatorName := bgShardedCluster.Name + "-coordinator"

	if !reflect.DeepEqual(workerClusters, bgShardedCluster.Status.WorkerClusters) ||
		bgShardedCluster.Status.CoordinatorCluster != coordinatorName ||
		bgShardedCluster.Status.Status != "Ready" {

		bgShardedCluster.Status.Status = "Ready"
		bgShardedCluster.Status.CoordinatorCluster = coordinatorName
		bgShardedCluster.Status.WorkerClusters = workerClusters

		return r.Status().Update(ctx, bgShardedCluster)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BGShardedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bestgresv1.BGShardedCluster{}).
		Owns(&bestgresv1.BGCluster{}).
		Complete(r)
}