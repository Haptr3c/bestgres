package controllers

import (
	"context"
	"encoding/base64"
	"reflect"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bestgresv1 "bestgres/api/v1"
)

// BGClusterReconciler reconciles a BGCluster object
type BGClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *BGClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log, _ := logr.FromContext(ctx)

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
	if err := r.reconcileEndpoints(ctx, bgCluster); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileService(ctx, bgCluster); err != nil {
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
	if err := r.reconcileClusterRole(ctx, bgCluster); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileClusterRoleBinding(ctx, bgCluster); err != nil {
		return ctrl.Result{}, err
	}

	// Update status
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(bgCluster.Namespace),
		client.MatchingLabels(labelsForBGCluster(bgCluster.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "BGCluster.Namespace", bgCluster.Namespace, "BGCluster.Name", bgCluster.Name)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	if !reflect.DeepEqual(podNames, bgCluster.Status.Nodes) {
		bgCluster.Status.Nodes = podNames
		err := r.Status().Update(ctx, bgCluster)
		if err != nil {
			log.Error(err, "Failed to update BGCluster status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *BGClusterReconciler) reconcileHeadlessService(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bgCluster.Name + "-config",
			Namespace: bgCluster.Namespace,
			Labels:    labelsForBGCluster(bgCluster.Name),
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labelsForBGCluster(bgCluster.Name),
		},
	}
	return ctrl.SetControllerReference(bgCluster, svc, r.Scheme)
}

func (r *BGClusterReconciler) reconcileStatefulSet(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	labels := labelsForBGCluster(bgCluster.Name)
	replicas := bgCluster.Spec.Instances

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bgCluster.Name,
			Namespace: bgCluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			ServiceName: bgCluster.Name,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: bgCluster.Name,
					Containers: []corev1.Container{
						{
							Name:            bgCluster.Name,
							Image:           "patroni:latest", // Replace with your Patroni image
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8008, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 5432, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "pgdata", MountPath: "/home/postgres/pgdata"},
							},
							Env: []corev1.EnvVar{
								{Name: "PATRONI_KUBERNETES_POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}},
								{Name: "PATRONI_KUBERNETES_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
								{Name: "PATRONI_KUBERNETES_BYPASS_API_SERVICE", Value: "true"},
								{Name: "PATRONI_KUBERNETES_USE_ENDPOINTS", Value: "true"},
								{Name: "PATRONI_KUBERNETES_LABELS", Value: "{application: patroni, cluster-name: " + bgCluster.Name + "}"},
								{Name: "PATRONI_SUPERUSER_USERNAME", Value: "postgres"},
								{Name: "PATRONI_SUPERUSER_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: bgCluster.Name}, Key: "superuser-password"}}},
								{Name: "PATRONI_REPLICATION_USERNAME", Value: "standby"},
								{Name: "PATRONI_REPLICATION_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: bgCluster.Name}, Key: "replication-password"}}},
								{Name: "PATRONI_SCOPE", Value: bgCluster.Name},
								{Name: "PATRONI_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
								{Name: "PATRONI_POSTGRESQL_DATA_DIR", Value: "/home/postgres/pgdata/pgroot/data"},
								{Name: "PATRONI_POSTGRESQL_PGPASS", Value: "/tmp/pgpass"},
								{Name: "PATRONI_POSTGRESQL_LISTEN", Value: "0.0.0.0:5432"},
								{Name: "PATRONI_RESTAPI_LISTEN", Value: "0.0.0.0:8008"},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Scheme: corev1.URISchemeHTTP,
										Path:   "/readiness",
										Port:   intstr.FromInt(8008),
									},
								},
								InitialDelaySeconds: 3,
								PeriodSeconds:      10,
								TimeoutSeconds:     5,
								SuccessThreshold:   1,
								FailureThreshold:   3,
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pgdata",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse(bgCluster.Spec.PersistentVolumeSize),
							},
						},
						StorageClassName: &bgCluster.Spec.StorageClass,
					},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(bgCluster, sts, r.Scheme); err != nil {
		return err
	}

	return r.Create(ctx, sts)
}

func (r *BGClusterReconciler) reconcileEndpoints(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bgCluster.Name,
			Namespace: bgCluster.Namespace,
			Labels:    labelsForBGCluster(bgCluster.Name),
		},
		Subsets: []corev1.EndpointSubset{},
	}
	return ctrl.SetControllerReference(bgCluster, endpoints, r.Scheme)
}

func (r *BGClusterReconciler) reconcileService(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bgCluster.Name,
			Namespace: bgCluster.Namespace,
			Labels:    labelsForBGCluster(bgCluster.Name),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port:       5432,
				TargetPort: intstr.FromInt(5432),
			}},
			Selector: labelsForBGCluster(bgCluster.Name),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
	return ctrl.SetControllerReference(bgCluster, svc, r.Scheme)
}

func (r *BGClusterReconciler) reconcileReplicaService(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bgCluster.Name + "-repl",
			Namespace: bgCluster.Namespace,
			Labels:    labelsForBGCluster(bgCluster.Name),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Port:       5432,
				TargetPort: intstr.FromInt(5432),
			}},
			Selector: map[string]string{
				"application":  "patroni",
				"cluster-name": bgCluster.Name,
				"role":         "replica",
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	return ctrl.SetControllerReference(bgCluster, svc, r.Scheme)
}

func (r *BGClusterReconciler) reconcileSecret(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bgCluster.Name,
			Namespace: bgCluster.Namespace,
			Labels:    labelsForBGCluster(bgCluster.Name),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"superuser-password":   []byte(base64.StdEncoding.EncodeToString([]byte(bgCluster.Spec.SuperuserPassword))),
			"replication-password": []byte(base64.StdEncoding.EncodeToString([]byte(bgCluster.Spec.ReplicationPassword))),
		},
	}
	return ctrl.SetControllerReference(bgCluster, secret, r.Scheme)
}

func (r *BGClusterReconciler) reconcileServiceAccount(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bgCluster.Name,
			Namespace: bgCluster.Namespace,
		},
	}
	return ctrl.SetControllerReference(bgCluster, sa, r.Scheme)
}

func (r *BGClusterReconciler) reconcileRole(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bgCluster.Name,
			Namespace: bgCluster.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"create", "get", "list", "patch", "update", "watch", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints"},
				Verbs:     []string{"create", "get", "list", "patch", "update", "watch", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "patch", "update", "watch"},
			},
		},
	}
	return ctrl.SetControllerReference(bgCluster, role, r.Scheme)
}

func (r *BGClusterReconciler) reconcileRoleBinding(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bgCluster.Name,
			Namespace: bgCluster.Namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     bgCluster.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      bgCluster.Name,
				Namespace: bgCluster.Namespace,
			},
		},
	}
	return ctrl.SetControllerReference(bgCluster, roleBinding, r.Scheme)
}

func (r *BGClusterReconciler) reconcileClusterRole(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "patroni-" + bgCluster.Name + "-k8s-ep-access",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"endpoints"},
				ResourceNames: []string{"kubernetes"},
				Verbs:         []string{"get"},
			},
		},
	}
	return ctrl.SetControllerReference(bgCluster, clusterRole, r.Scheme)
}

func (r *BGClusterReconciler) reconcileClusterRoleBinding(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "patroni-" + bgCluster.Name + "-k8s-ep-access",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "patroni-" + bgCluster.Name + "-k8s-ep-access",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      bgCluster.Name,
				Namespace: bgCluster.Namespace,
			},
		},
	}
	return ctrl.SetControllerReference(bgCluster, clusterRoleBinding, r.Scheme)
}

func labelsForBGCluster(name string) map[string]string {
	return map[string]string{
		"application":  "patroni",
		"cluster-name": name,
	}
}

func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

func (r *BGClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&bestgresv1.BGCluster{}).
        Owns(&appsv1.StatefulSet{}).
        Owns(&corev1.Service{}).
        Owns(&corev1.Endpoints{}).
        Owns(&corev1.Secret{}).
        Owns(&corev1.ServiceAccount{}).
        Owns(&rbacv1.Role{}).
        Owns(&rbacv1.RoleBinding{}).
        Owns(&rbacv1.ClusterRole{}).
        Owns(&rbacv1.ClusterRoleBinding{}).
        Complete(r)
}