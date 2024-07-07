package controllers

import (
	bestgresv1 "bestgres/api/v1"
	"context"
	"encoding/base64"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)


func (r *BGClusterReconciler) reconcileSecret(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
    log := ctrl.LoggerFrom(ctx)
    secret := &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      bgCluster.Name,
            Namespace: bgCluster.Namespace,
            Labels:    labelsForBGCluster(bgCluster.Name),
        },
        Type: corev1.SecretTypeOpaque,
        Data: map[string][]byte{},
    }

    // Retrieve the existing secret if it exists
    existingSecret := &corev1.Secret{}
    err := r.Get(ctx, client.ObjectKey{Name: bgCluster.Name, Namespace: bgCluster.Namespace}, existingSecret)
    if err == nil {
        secret.Data = existingSecret.Data
    }

    // Generate passwords if they don't exist
    if _, exists := secret.Data["superuser-password"]; !exists {
        password, err := generateRandomPassword(16)
        if err != nil {
            return err
        }
        secret.Data["superuser-password"] = []byte(base64.StdEncoding.EncodeToString([]byte(password)))
    }
    if _, exists := secret.Data["replication-password"]; !exists {
        password, err := generateRandomPassword(16)
        if err != nil {
            return err
        }
        secret.Data["replication-password"] = []byte(base64.StdEncoding.EncodeToString([]byte(password)))
    }
    if _, exists := secret.Data["admin-password"]; !exists {
        password, err := generateRandomPassword(16)
        if err != nil {
            return err
        }
        secret.Data["admin-password"] = []byte(base64.StdEncoding.EncodeToString([]byte(password)))
    }
    if _, exists := secret.Data["admin-user"]; !exists {
        user, err := generateRandomPassword(8)
        if err != nil {
            return err
        }
        secret.Data["admin-user"] = []byte(base64.StdEncoding.EncodeToString([]byte(user)))
    }

    // Create or update the secret
    if err := r.Update(ctx, secret); err != nil {
        if client.IgnoreNotFound(err) == nil {
            log.Info("Creating new secret")
            if err := r.Create(ctx, secret); err != nil {
                return err
            }
        } else {
            return err
        }
    } else {
        log.Info("Updating existing secret")
    }

    return nil
}
