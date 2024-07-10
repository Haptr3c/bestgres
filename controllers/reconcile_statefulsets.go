package controllers

import (
	bestgresv1 "bestgres/api/v1"
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *BGClusterReconciler) reconcileStatefulSet(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	log := ctrl.LoggerFrom(ctx)
	sts := r.createStatefulSetObject(bgCluster)

	if err := ctrl.SetControllerReference(bgCluster, sts, r.Scheme); err != nil {
		return err
	}

	foundSts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, foundSts)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.createStatefulSet(ctx, sts)
		}
		log.Error(err, "Failed to get StatefulSet")
		return err
	}

	return r.updateStatefulSet(ctx, sts, foundSts)
}

func (r *BGClusterReconciler) createStatefulSetObject(bgCluster *bestgresv1.BGCluster) *appsv1.StatefulSet {
	labels := r.getLabelsAndAnnotations(bgCluster)
	replicas := bgCluster.Spec.Instances

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        bgCluster.Name,
			Namespace:   bgCluster.Namespace,
			Labels:      labels,
			Annotations: labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: bgCluster.Name,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template:             r.createPodTemplateSpec(bgCluster),
			VolumeClaimTemplates: r.createVolumeClaimTemplates(bgCluster),
		},
	}
}

func (r *BGClusterReconciler) createPodTemplateSpec(bgCluster *bestgresv1.BGCluster) corev1.PodTemplateSpec {
	labels := r.getLabelsAndAnnotations(bgCluster)

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels,
			Annotations: labels,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: bgCluster.Name,
			Containers:         []corev1.Container{r.createMainContainer(bgCluster)},
			InitContainers:     []corev1.Container{r.createInitContainer(bgCluster)},
		},
	}
}

func (r *BGClusterReconciler) createMainContainer(bgCluster *bestgresv1.BGCluster) corev1.Container {
	return corev1.Container{
		Name:            bgCluster.Name,
		Image:           bgCluster.Spec.Image.Tag,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports:           r.createContainerPorts(),
		Command:         []string{"/app/controller"},
		VolumeMounts:    r.createVolumeMounts(),
		Env:             r.createEnvironmentVariables(bgCluster),
	}
}

func (r *BGClusterReconciler) createInitContainer(bgCluster *bestgresv1.BGCluster) corev1.Container {
	return corev1.Container{
		Name:            bgCluster.Name + "-init",
		Image:           getOperatorImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports:           []corev1.ContainerPort{{ContainerPort: 8008, Protocol: corev1.ProtocolTCP}},
		VolumeMounts:    []corev1.VolumeMount{{Name: "controller", MountPath: "/app"}},
		Env:             []corev1.EnvVar{{Name: "MODE", Value: "init"}},
	}
}

func (r *BGClusterReconciler) createContainerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{ContainerPort: 8008, Protocol: corev1.ProtocolTCP},
		{ContainerPort: 5432, Protocol: corev1.ProtocolTCP},
	}
}

func (r *BGClusterReconciler) createVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: "pgdata", MountPath: "/home/postgres/pgdata"},
		{Name: "controller", MountPath: "/app"},
	}
}

func (r *BGClusterReconciler) createEnvironmentVariables(bgCluster *bestgresv1.BGCluster) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "MODE", Value: "controller"},
		{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
		{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}},
		{Name: "POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
		{Name: "SCOPE", Value: bgCluster.Name},
		{Name: "DCS_ENABLE_KUBERNETES_API", Value: "true"},
		{Name: "PATRONI_KUBERNETES_USE_ENDPOINTS", Value: "false"},
		{Name: "PATRONI_LOG_LEVEL", Value: bgCluster.Spec.PatroniLogLevel},
		// TODO Test and then set this
		// {Name: "PATRONI_KUBERNETES_BYPASS_API_SERVICE", Value: "false"},
		{Name: "BGMON_LISTEN_IP", Value: "*"},
		{Name: "KUBERNETES_USE_CONFIGMAPS", Value: "true"},
		{Name: "KUBERNETES_SCOPE_LABEL", Value: "cluster-name"},
		{Name: "KUBERNETES_ROLE_LABEL", Value: "role"},
		{Name: "PGUSER_ADMIN", Value: "admin"},
		{Name: "PGPASSWORD_SUPERUSER", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: bgCluster.Name}, Key: "superuser-password"}}},
		{Name: "PGPASSWORD_STANDBY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: bgCluster.Name}, Key: "replication-password"}}},
		{Name: "PGPASSWORD_ADMIN", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: bgCluster.Name}, Key: "admin-password"}}},
		{Name: "PGROOT", Value: "/home/postgres/pgdata/pgroot"},
		{Name: "SPILO_CONFIGURATION", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: bgCluster.Name + "-postgres-config"}, Key: "postgres.yaml"}}},
	}
}

func (r *BGClusterReconciler) createVolumeClaimTemplates(bgCluster *bestgresv1.BGCluster) []corev1.PersistentVolumeClaim {
	return []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "pgdata"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(bgCluster.Spec.VolumeSpec.PersistentVolumeSize),
					},
				},
				StorageClassName: &bgCluster.Spec.VolumeSpec.StorageClass,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "controller"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("100Mi"),
					},
				},
				StorageClassName: &bgCluster.Spec.VolumeSpec.StorageClass,
			},
		},
	}
}

func (r *BGClusterReconciler) getLabelsAndAnnotations(bgCluster *bestgresv1.BGCluster) map[string]string {
	labels := map[string]string{
		"application":  "bestgres",
		"cluster-name": bgCluster.Name,
	}

	if bgCluster.Labels["bgcluster.bestgres.io/part-of"] != "" {
		labels["bgcluster.bestgres.io/part-of"] = bgCluster.Labels["bgcluster.bestgres.io/part-of"]
	}

	if bgCluster.Labels["bgcluster.bestgres.io/role"] != "" {
		labels["bgcluster.bestgres.io/role"] = bgCluster.Labels["bgcluster.bestgres.io/role"]
	}

	return labels
}

func (r *BGClusterReconciler) createStatefulSet(ctx context.Context, sts *appsv1.StatefulSet) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
	return r.Create(ctx, sts)
}

func (r *BGClusterReconciler) updateStatefulSet(ctx context.Context, sts, foundSts *appsv1.StatefulSet) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Updating existing StatefulSet", "StatefulSet.Namespace", foundSts.Namespace, "StatefulSet.Name", foundSts.Name)
	sts.ResourceVersion = foundSts.ResourceVersion
	return r.Update(ctx, sts)
}