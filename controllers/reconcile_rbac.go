package controllers

import (
	bestgresv1 "bestgres/api/v1"
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)


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
                Resources: []string{"configmaps", "endpoints"},
                Verbs:     []string{"create", "get", "list", "patch", "update", "watch", "delete"},
            },
            {
                APIGroups: []string{""},
                Resources: []string{"pods"},
                Verbs:     []string{"get", "list", "patch", "update", "watch"},
            },
            {
                APIGroups: []string{""},
                Resources: []string{"services"},
                Verbs:     []string{"create"},
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
                Verbs:     []string{"get", "update"},
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
