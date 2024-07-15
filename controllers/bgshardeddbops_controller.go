// bgshardeddbops_controller.go

package controllers

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bestgresv1 "bestgres/api/v1"
)

// BGShardedDbOpsReconciler reconciles a BGShardedDbOps object
// This struct contains the necessary components to reconcile BGShardedDbOps resources
type BGShardedDbOpsReconciler struct {
	// Client is a split client that reads objects from the cache and writes to the apiserver
	client.Client
	// Scheme defines methods for serializing and deserializing API objects
	Scheme *runtime.Scheme
	// Namespace is the namespace in which this controller operates
	Namespace string
}

//+kubebuilder:rbac:groups=bestgres.io,resources=bgshardeddbops,verbs=get;list;watch;create;update;patch;delete,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgshardeddbops/status,verbs=get;update;patch,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgshardeddbops/finalizers,verbs=update,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgshardedclusters,verbs=get;list;watch,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgclusters,verbs=get;list;watch,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgdbops,verbs=get;list;watch;create;update;patch;delete,namespace="{{ .Release.Namespace }}"

// SetupWithManager sets up the controller with the Manager.
// This function is called when the controller is initialized to set up the Manager.
func (r *BGShardedDbOpsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Watch for changes to BGShardedDbOps resources
		For(&bestgresv1.BGShardedDbOps{}).
		Complete(r)
}