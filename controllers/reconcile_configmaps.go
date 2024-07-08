package controllers

import (
	"context"
	"fmt"

	bestgresv1 "bestgres/api/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *BGClusterReconciler) reconcileConfigMaps(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	log := ctrl.LoggerFrom(ctx)

	configMapNames := []string{
		bgCluster.Name + "-config",
		bgCluster.Name + "-leader",
	}

	for _, cmName := range configMapNames {
		cm := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: bgCluster.Namespace}, cm)

		if err != nil {
			if errors.IsNotFound(err) {
				// ConfigMap doesn't exist yet, which is fine
				log.Info(fmt.Sprintf("ConfigMap %s not found, skipping", cmName))
				continue
			}
			// For other errors, return the error
			return err
		}

		// Check if the BGCluster is already the owner
		if isOwnedByBGCluster(cm, bgCluster) {
			log.Info(fmt.Sprintf("ConfigMap %s is already owned by BGCluster, skipping", cmName))
			continue
		}

		// Update the owner reference
		if err := ctrl.SetControllerReference(bgCluster, cm, r.Scheme); err != nil {
			log.Error(err, fmt.Sprintf("Failed to set controller reference for ConfigMap %s", cmName))
			return err
		}

		// Update the ConfigMap
		if err := r.Update(ctx, cm); err != nil {
			log.Error(err, fmt.Sprintf("Failed to update ConfigMap %s", cmName))
			return err
		}

		log.Info(fmt.Sprintf("Successfully updated owner reference for ConfigMap %s", cmName))
	}

	return nil
}
