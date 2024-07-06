// bgdbops_controller.go

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bestgresv1 "bestgres/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
)

// BGDbOpsReconciler reconciles a BGDbOps object
type BGDbOpsReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=bestgres.io,resources=bgdbops,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=bestgres.io,resources=bgdbops/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=bestgres.io,resources=bgdbops/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;update;patch

func (r *BGDbOpsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log, _ := logr.FromContext(ctx)

	// Fetch the BGDbOps instance
	bgDbOps := &bestgresv1.BGDbOps{}
	err := r.Get(ctx, req.NamespacedName, bgDbOps)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Unable to fetch BGDbOps")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if the operation is already completed
	if bgDbOps.Status.Status == "Completed" {
		return ctrl.Result{}, nil
	}

	// Fetch the associated BGCluster
	bgCluster := &bestgresv1.BGCluster{}
	err = r.Get(ctx, types.NamespacedName{Name: bgDbOps.Spec.BGCluster, Namespace: bgDbOps.Namespace}, bgCluster)
	if err != nil {
		log.Error(err, "Unable to fetch associated BGCluster")
		return ctrl.Result{}, err
	}

	// Perform the requested operation
	switch bgDbOps.Spec.Op {
	case "benchmark":
		err = r.performBenchmark(ctx, bgDbOps, bgCluster)
	case "repack":
		err = r.performRepack(ctx, bgDbOps, bgCluster)
	case "restart":
		err = r.performRestart(ctx, bgDbOps, bgCluster)
	case "vacuum":
		err = r.performVacuum(ctx, bgDbOps, bgCluster)
	default:
		err = fmt.Errorf("unknown operation: %s", bgDbOps.Spec.Op)
	}

	if err != nil {
		log.Error(err, "Failed to perform operation", "Operation", bgDbOps.Spec.Op)
		bgDbOps.Status.Status = "Failed"
		bgDbOps.Status.Retries++
	} else {
		bgDbOps.Status.Status = "Completed"
	}

	// Update BGDbOps status
	if updateErr := r.Status().Update(ctx, bgDbOps); updateErr != nil {
		log.Error(updateErr, "Failed to update BGDbOps status")
		return ctrl.Result{}, updateErr
	}

	// If operation failed and max retries not reached, requeue
	if err != nil && bgDbOps.Status.Retries < bgDbOps.Spec.MaxRetries {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	return ctrl.Result{}, nil
}

func (r *BGDbOpsReconciler) performBenchmark(ctx context.Context, bgDbOps *bestgresv1.BGDbOps, bgCluster *bestgresv1.BGCluster) error {
	// For benchmarking, we could create a separate Job that runs pgbench
	// Here we'll just update the StatefulSet to simulate the operation
	return r.updateStatefulSetWithOperation(ctx, bgCluster, "benchmark")
}

func (r *BGDbOpsReconciler) performRepack(ctx context.Context, bgDbOps *bestgresv1.BGDbOps, bgCluster *bestgresv1.BGCluster) error {
	return r.updateStatefulSetWithOperation(ctx, bgCluster, "repack")
}

func (r *BGDbOpsReconciler) performRestart(ctx context.Context, bgDbOps *bestgresv1.BGDbOps, bgCluster *bestgresv1.BGCluster) error {
	return r.updateStatefulSetWithOperation(ctx, bgCluster, "restart")
}

func (r *BGDbOpsReconciler) performVacuum(ctx context.Context, bgDbOps *bestgresv1.BGDbOps, bgCluster *bestgresv1.BGCluster) error {
	return r.updateStatefulSetWithOperation(ctx, bgCluster, "vacuum")
}

func (r *BGDbOpsReconciler) updateStatefulSetWithOperation(ctx context.Context, bgCluster *bestgresv1.BGCluster, operation string) error {
	// Fetch the StatefulSet
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: bgCluster.Name, Namespace: bgCluster.Namespace}, sts)
	if err != nil {
		return err
	}

	// Update the StatefulSet's annotations to trigger the operation
	if sts.Spec.Template.ObjectMeta.Annotations == nil {
		sts.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}
	sts.Spec.Template.ObjectMeta.Annotations["bestgres.io/operation"] = operation
	sts.Spec.Template.ObjectMeta.Annotations["bestgres.io/operation-timestamp"] = time.Now().Format(time.RFC3339)

	// Update the StatefulSet
	return r.Update(ctx, sts)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BGDbOpsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bestgresv1.BGDbOps{}).
        Complete(r)
}