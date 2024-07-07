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
							Image:           bgCluster.Spec.Image.Tag,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8008, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 5432, Protocol: corev1.ProtocolTCP},
							},
                            Command: []string{"/app/controller"},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "pgdata", MountPath: "/home/postgres/pgdata"},
								{Name: "controller", MountPath: "/app", },
							},
							Env: []corev1.EnvVar{
                                {Name: "MODE", Value: "controller"},
                                {Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
                                {Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}}},
                                {Name: "POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
                                {Name: "SCOPE", Value: bgCluster.Name},
                                {Name: "DCS_ENABLE_KUBERNETES_API", Value: "true"},
								{Name: "PATRONI_KUBERNETES_USE_ENDPOINTS", Value: "true"},
								{Name: "PATRONI_LOG_LEVEL", Value: bgCluster.Spec.PatroniLogLevel},
                                {Name: "KUBERNETES_SCOPE_LABEL", Value: "cluster-name"},
								{Name: "KUBERNETES_ROLE_LABEL", Value: "role"},
								{Name: "PGUSER_ADMIN", Value: "admin-user"},
								{Name: "PGPASSWORD_SUPERUSER", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: bgCluster.Name}, Key: "superuser-password"}}},
								{Name: "PGPASSWORD_STANDBY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: bgCluster.Name}, Key: "replication-password"}}},
								{Name: "PGPASSWORD_ADMIN", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: bgCluster.Name}, Key: "admin-password"}}},
                                {Name: "PGROOT", Value: "/home/postgres/pgdata/pgroot"},
                                {
                                    Name: "SPILO_CONFIGURATION",
                                    Value: `bootstrap:
  initdb:
    - auth-host: md5
    - auth-local: trust`,
                                },
                            },
							// ReadinessProbe: &corev1.Probe{
							// 	ProbeHandler: corev1.ProbeHandler{
							// 		HTTPGet: &corev1.HTTPGetAction{
							// 			Scheme: corev1.URISchemeHTTP,
							// 			Path:   "/readiness",
							// 			Port:   intstr.FromInt(8008),
							// 		},
							// 	},
							// 	InitialDelaySeconds: 3,
							// 	PeriodSeconds:      10,
							// 	TimeoutSeconds:     5,
							// 	SuccessThreshold:   1,
							// 	FailureThreshold:   3,
							// },
						},
					},
                    InitContainers: []corev1.Container{
						{
							Name:            bgCluster.Name + "-init",
							Image:           getOperatorImage(),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8008, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "controller", MountPath: "/app"},
							},
							Env: []corev1.EnvVar{
								{Name: "MODE", Value: "init"},
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
                {
                    ObjectMeta: metav1.ObjectMeta{
                        Name: "controller",
                    },
                    Spec: corev1.PersistentVolumeClaimSpec{
                        AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
                        Resources: corev1.VolumeResourceRequirements{
                            Requests: corev1.ResourceList{
                                corev1.ResourceStorage: resource.MustParse("100Mi"),
                            },
                        },
                        StorageClassName: &bgCluster.Spec.StorageClass,
                    },
                },
			},
		},
	}

	// Set BGCluster instance as the owner and controller
	if err := ctrl.SetControllerReference(bgCluster, sts, r.Scheme); err != nil {
		return err
	}

	// Check if the StatefulSet already exists
	foundSts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, foundSts)
	if err != nil {
		if errors.IsNotFound(err) {
			// StatefulSet does not exist, create a new one
			log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
			err = r.Create(ctx, sts)
			if err != nil {
				log.Error(err, "Failed to create new StatefulSet", "StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
				return err
			}
		} else {
			// Error reading the object - requeue the request.
			log.Error(err, "Failed to get StatefulSet")
			return err
		}
	} else {
		// StatefulSet already exists, update it
		log.Info("Updating existing StatefulSet", "StatefulSet.Namespace", foundSts.Namespace, "StatefulSet.Name", foundSts.Name)
		sts.ResourceVersion = foundSts.ResourceVersion
		err = r.Update(ctx, sts)
		if err != nil {
			log.Error(err, "Failed to update StatefulSet", "StatefulSet.Namespace", sts.Namespace, "StatefulSet.Name", sts.Name)
			return err
		}
	}

	return nil
}
