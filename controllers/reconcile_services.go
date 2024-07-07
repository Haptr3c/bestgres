package controllers

import (
	bestgresv1 "bestgres/api/v1"
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)


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
