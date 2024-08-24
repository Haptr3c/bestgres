package controllers

import (
	"context"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bestgresv1 "bestgres/api/v1"
)

// BGClusterReconciler reconciles a BGCluster object
type BGClusterReconciler struct {
    client.Client
    Scheme *runtime.Scheme
	Namespace string
}

//+kubebuilder:rbac:groups=bestgres.io,resources=bgclusters,verbs=get;list;watch;create;update;patch;delete,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgclusters/status,verbs=get;update;patch,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=bestgres.io,resources=bgclusters/finalizers,verbs=update,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=core,resources=pods;services;endpoints;secrets;serviceaccounts;configmaps,verbs=get;list;watch;create;update;patch;delete,namespace="{{ .Release.Namespace }}"
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete,namespace="{{ .Release.Namespace }}"

func (r *BGClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := ctrl.LoggerFrom(ctx)

    bgCluster := &bestgresv1.BGCluster{}
    err := r.Get(ctx, req.NamespacedName, bgCluster)
    if err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Create or update resources
    if err := r.reconcileHeadlessService(ctx, bgCluster); err != nil {
        return ctrl.Result{}, err
    }
    if err := r.reconcileStatefulSet(ctx, bgCluster); err != nil {
        return ctrl.Result{}, err
    }
    if err := r.reconcileService(ctx, bgCluster); err != nil {
        return ctrl.Result{}, err
    }
	if err := r.reconcileConfigMaps(ctx, bgCluster); err != nil {
		return ctrl.Result{}, err
	}
    if err := r.reconcileReplicaService(ctx, bgCluster); err != nil {
        return ctrl.Result{}, err
    }
    if err := r.reconcileSecret(ctx, bgCluster); err != nil {
        return ctrl.Result{}, err
    }
    if err := r.reconcileServiceAccount(ctx, bgCluster); err != nil {
        return ctrl.Result{}, err
    }
    if err := r.reconcileRole(ctx, bgCluster); err != nil {
        return ctrl.Result{}, err
    }
    if err := r.reconcileRoleBinding(ctx, bgCluster); err != nil {
        return ctrl.Result{}, err
    }

    // Update status
    podList := &corev1.PodList{}
    listOpts := []client.ListOption{
        client.InNamespace(req.Namespace),
        client.MatchingLabels(labelsForBGCluster(bgCluster.Name)),
    }
    if err = r.List(ctx, podList, listOpts...); err != nil {
        log.Error(err, "Failed to list pods", "BGCluster.Namespace", bgCluster.Namespace, "BGCluster.Name", bgCluster.Name)
        return ctrl.Result{}, err
    }
    podNames := getPodNames(podList.Items)

    // TODO test this, might break stuff
    bgCluster = refreshContext(bgCluster, r.Client)

    if !reflect.DeepEqual(podNames, bgCluster.Status.Nodes) {
        bgCluster.Status.Nodes = podNames
        err := r.Status().Update(ctx, bgCluster)
        if err != nil {
            log.Error(err, "Error in bgCluster.Status.Update")
            return ctrl.Result{}, err
        }
    }

    return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BGClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bestgresv1.BGCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}
