// bgdbops_controller.go

package controllers

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bestgresv1 "bestgres/api/v1"
)

// BGDbOpsReconciler reconciles a BGDbOps object
type BGDbOpsReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Namespace string
}

//+kubebuilder:rbac:groups=bestgres.io,resources=bgdbops,verbs=get;list;watch;create;update;patch;delete,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgdbops/status,verbs=get;update;patch,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgdbops/finalizers,verbs=update,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgclusters,verbs=get;list;watch;update;patch,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch,namespace="{{ .Release.Namespace }}"

// SetupWithManager sets up the controller with the Manager.
func (r *BGDbOpsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bestgresv1.BGDbOps{}).
		Complete(r)
}
