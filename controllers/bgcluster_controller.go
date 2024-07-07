package controllers

import (
	"context"
	"encoding/base64"
	"os"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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

func getOperatorImage() (string) {
    imageName := os.Getenv("OPERATOR_IMAGE")
    if imageName == "" {
        imageName = "bestgres/operator:latest"
    }
    return imageName
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
							Image:           bgCluster.Spec.Patroni.Image,
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
                                {Name: "POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
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

func (r *BGClusterReconciler) reconcileHeadlessService(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
    log := ctrl.LoggerFrom(ctx)
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
    
    if err := ctrl.SetControllerReference(bgCluster, svc, r.Scheme); err != nil {
        return err
    }

    foundSvc := &corev1.Service{}
    err := r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, foundSvc)
    if err != nil {
        if errors.IsNotFound(err) {
            log.Info("Creating a new Headless Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
            err = r.Create(ctx, svc)
            if err != nil {
                log.Error(err, "Failed to create new Headless Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
                return err
            }
        } else {
            log.Error(err, "Failed to get Headless Service")
            return err
        }
    } else {
        log.Info("Updating existing Headless Service", "Service.Namespace", foundSvc.Namespace, "Service.Name", foundSvc.Name)
        svc.ResourceVersion = foundSvc.ResourceVersion
        err = r.Update(ctx, svc)
        if err != nil {
            log.Error(err, "Failed to update Headless Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
            return err
        }
    }

    return nil
}

func (r *BGClusterReconciler) reconcileEndpoints(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
    log := ctrl.LoggerFrom(ctx)
    endpoints := &corev1.Endpoints{
        ObjectMeta: metav1.ObjectMeta{
            Name:      bgCluster.Name,
            Namespace: bgCluster.Namespace,
            Labels:    labelsForBGCluster(bgCluster.Name),
        },
        Subsets: []corev1.EndpointSubset{},
    }
    
    if err := ctrl.SetControllerReference(bgCluster, endpoints, r.Scheme); err != nil {
        return err
    }

    foundEndpoints := &corev1.Endpoints{}
    err := r.Get(ctx, types.NamespacedName{Name: endpoints.Name, Namespace: endpoints.Namespace}, foundEndpoints)
    if err != nil {
        if errors.IsNotFound(err) {
            log.Info("Creating a new Endpoints", "Endpoints.Namespace", endpoints.Namespace, "Endpoints.Name", endpoints.Name)
            err = r.Create(ctx, endpoints)
            if err != nil {
                log.Error(err, "Failed to create new Endpoints", "Endpoints.Namespace", endpoints.Namespace, "Endpoints.Name", endpoints.Name)
                return err
            }
        } else {
            log.Error(err, "Failed to get Endpoints")
            return err
        }
    } else {
        log.Info("Updating existing Endpoints", "Endpoints.Namespace", foundEndpoints.Namespace, "Endpoints.Name", foundEndpoints.Name)
        endpoints.ResourceVersion = foundEndpoints.ResourceVersion
        err = r.Update(ctx, endpoints)
        if err != nil {
            log.Error(err, "Failed to update Endpoints", "Endpoints.Namespace", endpoints.Namespace, "Endpoints.Name", endpoints.Name)
            return err
        }
    }

    return nil
}

func (r *BGClusterReconciler) reconcileService(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
    log := ctrl.LoggerFrom(ctx)
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
    
    if err := ctrl.SetControllerReference(bgCluster, svc, r.Scheme); err != nil {
        return err
    }

    foundSvc := &corev1.Service{}
    err := r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, foundSvc)
    if err != nil {
        if errors.IsNotFound(err) {
            log.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
            err = r.Create(ctx, svc)
            if err != nil {
                log.Error(err, "Failed to create new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
                return err
            }
        } else {
            log.Error(err, "Failed to get Service")
            return err
        }
    } else {
        log.Info("Updating existing Service", "Service.Namespace", foundSvc.Namespace, "Service.Name", foundSvc.Name)
        svc.ResourceVersion = foundSvc.ResourceVersion
        err = r.Update(ctx, svc)
        if err != nil {
            log.Error(err, "Failed to update Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
            return err
        }
    }

    return nil
}

func (r *BGClusterReconciler) reconcileReplicaService(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
    log := ctrl.LoggerFrom(ctx)
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
    
    if err := ctrl.SetControllerReference(bgCluster, svc, r.Scheme); err != nil {
        return err
    }

    foundSvc := &corev1.Service{}
    err := r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, foundSvc)
    if err != nil {
        if errors.IsNotFound(err) {
            log.Info("Creating a new Replica Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
            err = r.Create(ctx, svc)
            if err != nil {
                log.Error(err, "Failed to create new Replica Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
                return err
            }
        } else {
            log.Error(err, "Failed to get Replica Service")
            return err
        }
    } else {
        log.Info("Updating existing Replica Service", "Service.Namespace", foundSvc.Namespace, "Service.Name", foundSvc.Name)
        svc.ResourceVersion = foundSvc.ResourceVersion
        err = r.Update(ctx, svc)
        if err != nil {
            log.Error(err, "Failed to update Replica Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
            return err
        }
    }

    return nil
}

func (r *BGClusterReconciler) reconcileSecret(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
    log := ctrl.LoggerFrom(ctx)
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
    
    if err := ctrl.SetControllerReference(bgCluster, secret, r.Scheme); err != nil {
        return err
    }

    foundSecret := &corev1.Secret{}
    err := r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, foundSecret)
    if err != nil {
        if errors.IsNotFound(err) {
            log.Info("Creating a new Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
            err = r.Create(ctx, secret)
            if err != nil {
                log.Error(err, "Failed to create new Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
                return err
            }
        } else {
            log.Error(err, "Failed to get Secret")
            return err
        }
    } else {
        log.Info("Updating existing Secret", "Secret.Namespace", foundSecret.Namespace, "Secret.Name", foundSecret.Name)
        secret.ResourceVersion = foundSecret.ResourceVersion
        err = r.Update(ctx, secret)
        if err != nil {
            log.Error(err, "Failed to update Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
            return err
        }
    }

    return nil
}

func (r *BGClusterReconciler) reconcileServiceAccount(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
    log := ctrl.LoggerFrom(ctx)
    sa := &corev1.ServiceAccount{
        ObjectMeta: metav1.ObjectMeta{
            Name:      bgCluster.Name,
            Namespace: bgCluster.Namespace,
        },
    }
    
    if err := ctrl.SetControllerReference(bgCluster, sa, r.Scheme); err != nil {
        return err
    }

    foundSA := &corev1.ServiceAccount{}
    err := r.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: sa.Namespace}, foundSA)
    if err != nil {
        if errors.IsNotFound(err) {
            log.Info("Creating a new ServiceAccount", "ServiceAccount.Namespace", sa.Namespace, "ServiceAccount.Name", sa.Name)
            err = r.Create(ctx, sa)
            if err != nil {
                log.Error(err, "Failed to create new ServiceAccount", "ServiceAccount.Namespace", sa.Namespace, "ServiceAccount.Name", sa.Name)
                return err
            }
        } else {
            log.Error(err, "Failed to get ServiceAccount")
            return err
        }
    } else {
        log.Info("ServiceAccount already exists", "ServiceAccount.Namespace", foundSA.Namespace, "ServiceAccount.Name", foundSA.Name)
    }

    return nil
}

func (r *BGClusterReconciler) reconcileRole(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
    log := ctrl.LoggerFrom(ctx)
    role := &rbacv1.Role{
        ObjectMeta: metav1.ObjectMeta{
            Name:      bgCluster.Name,
            Namespace: bgCluster.Namespace,
        },
        Rules: []rbacv1.PolicyRule{
            {
                APIGroups: []string{""},
                Resources: []string{"configmaps", "endpoints", "pods"},
                Verbs:     []string{"create", "get", "list", "patch", "update", "watch", "delete"},
            },
            {
                APIGroups: []string{"apps"},
                Resources: []string{"statefulsets"},
                ResourceNames: []string{bgCluster.Name},
                Verbs:     []string{"get"},
            },
            {
                APIGroups: []string{"bestgres.io"},
                Resources: []string{"bgclusters"},
                ResourceNames: []string{bgCluster.Name},
                Verbs:     []string{"get"},
            },
        },
    }
    
    if err := ctrl.SetControllerReference(bgCluster, role, r.Scheme); err != nil {
        return err
    }

    foundRole := &rbacv1.Role{}
    err := r.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, foundRole)
    if err != nil {
        if errors.IsNotFound(err) {
            log.Info("Creating a new Role", "Role.Namespace", role.Namespace, "Role.Name", role.Name)
            err = r.Create(ctx, role)
            if err != nil {
                log.Error(err, "Failed to create new Role", "Role.Namespace", role.Namespace, "Role.Name", role.Name)
                return err
            }
        } else {
            log.Error(err, "Failed to get Role")
            return err
		}
		} else {
			log.Info("Updating existing Role", "Role.Namespace", foundRole.Namespace, "Role.Name", foundRole.Name)
			role.ResourceVersion = foundRole.ResourceVersion
			err = r.Update(ctx, role)
			if err != nil {
				log.Error(err, "Failed to update Role", "Role.Namespace", role.Namespace, "Role.Name", role.Name)
				return err
			}
		}
	
		return nil
	}
	
func (r *BGClusterReconciler) reconcileRoleBinding(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	log := ctrl.LoggerFrom(ctx)
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
	
	if err := ctrl.SetControllerReference(bgCluster, roleBinding, r.Scheme); err != nil {
		return err
	}

	foundRoleBinding := &rbacv1.RoleBinding{}
	err := r.Get(ctx, types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, foundRoleBinding)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating a new RoleBinding", "RoleBinding.Namespace", roleBinding.Namespace, "RoleBinding.Name", roleBinding.Name)
			err = r.Create(ctx, roleBinding)
			if err != nil {
				log.Error(err, "Failed to create new RoleBinding", "RoleBinding.Namespace", roleBinding.Namespace, "RoleBinding.Name", roleBinding.Name)
				return err
			}
		} else {
			log.Error(err, "Failed to get RoleBinding")
			return err
		}
	} else {
		log.Info("Updating existing RoleBinding", "RoleBinding.Namespace", foundRoleBinding.Namespace, "RoleBinding.Name", foundRoleBinding.Name)
		roleBinding.ResourceVersion = foundRoleBinding.ResourceVersion
		err = r.Update(ctx, roleBinding)
		if err != nil {
			log.Error(err, "Failed to update RoleBinding", "RoleBinding.Namespace", roleBinding.Namespace, "RoleBinding.Name", roleBinding.Name)
			return err
		}
	}

	return nil
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

// SetupWithManager sets up the controller with the Manager.
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
		Complete(r)
}