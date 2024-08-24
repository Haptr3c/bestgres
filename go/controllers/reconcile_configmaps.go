package controllers

import (
	"context"
	"fmt"
	"time"

	bestgresv1 "bestgres/api/v1"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *BGClusterReconciler) reconcileConfigMaps(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	log := ctrl.LoggerFrom(ctx)

	// Create or update the Spilo configuration ConfigMap
	if err := r.reconcileSpiloConfigMap(ctx, bgCluster); err != nil {
		return err
	}

	configMapNames := []string{
		bgCluster.Name + "-config",
		bgCluster.Name + "-leader",
	}

	for _, cmName := range configMapNames {
		if err := wait.ExponentialBackoff(wait.Backoff{
			Duration: time.Second,
			Factor:   2,
			Jitter:   0.1,
			Steps:    5,
		}, func() (bool, error) {
			cm := &corev1.ConfigMap{}
			err := r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: bgCluster.Namespace}, cm)

			if err != nil {
				if errors.IsNotFound(err) {
					// ConfigMap doesn't exist yet, which is fine
					return true, nil
				}
				log.Error(err, "Failed to get ConfigMap", "ConfigMap", cmName)
				return false, err
			}

			// Check if the BGCluster is already the owner
			if isOwnedByBGCluster(cm, bgCluster) {
				return true, nil
			}

			// Update the owner reference
			if err := ctrl.SetControllerReference(bgCluster, cm, r.Scheme); err != nil {
				log.Error(err, "Failed to set controller reference", "ConfigMap", cmName)
				return false, err
			}

			// Update the ConfigMap in the API
			if err := r.Update(ctx, cm); err != nil {
				if errors.IsConflict(err) {
					// Conflict error, we'll retry
					log.Info("Conflict error when updating ConfigMap, retrying", "ConfigMap", cmName)
					return false, nil
				}
				log.Error(err, "Failed to update ConfigMap", "ConfigMap", cmName)
				return false, err
			}

			log.Info("Successfully updated owner reference", "ConfigMap", cmName)
			return true, nil
		}); err != nil {
			return fmt.Errorf("failed to reconcile ConfigMap %s after retries: %w", cmName, err)
		}
	}

	return nil
}

func (r *BGClusterReconciler) reconcileSpiloConfigMap(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	log := ctrl.LoggerFrom(ctx)
	configMapKey := "postgres.yaml"

	spiloConfig, err := r.createSpiloConfiguration()
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bgCluster.Name + "-postgres-config",
			Namespace: bgCluster.Namespace,
		},
		Data: map[string]string{
			configMapKey: spiloConfig,
		},
	}

	if err := ctrl.SetControllerReference(bgCluster, cm, r.Scheme); err != nil {
		return err
	}

	return wait.ExponentialBackoff(wait.Backoff{
		Duration: time.Second,
		Factor:   2,
		Jitter:   0.1,
		Steps:    5,
	}, func() (bool, error) {
		foundCm := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, foundCm)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Info("Creating ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
				if err := r.Create(ctx, cm); err != nil {
					log.Error(err, "Failed to create ConfigMap")
					return false, err
				}
				return true, nil
			}
			log.Error(err, "Failed to get ConfigMap")
			return false, err
		}

		if foundCm.Data[configMapKey] != cm.Data[configMapKey] {
			log.Info("Updating ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
			foundCm.Data = cm.Data
			if err := r.Update(ctx, foundCm); err != nil {
				if errors.IsConflict(err) {
					// Conflict error, we'll retry
					log.Info("Conflict error when updating ConfigMap, retrying")
					return false, nil
				}
				log.Error(err, "Failed to update ConfigMap")
				return false, err
			}
		}

		return true, nil
	})
}

func (r *BGClusterReconciler) createSpiloConfiguration() (string, error) {
	baseConfig := map[string]interface{}{
		"bootstrap": map[string]interface{}{
			"initdb": []map[string]string{
				{"auth-host": "md5"},
				{"auth-local": "trust"},
			},
		},
	}

	// TODO enable user-supplied configuration options
	// // Merge user-supplied configuration
	// if bgCluster.Spec.PostgresConf != nil {
	// 	for key, value := range bgCluster.Spec.PostgresConf {
	// 		baseConfig[key] = value
	// 	}
	// }
	
	//Convert the final configuration to a YAML string
	configBytes, err := yaml.Marshal(baseConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Spilo configuration: %w", err)
	}

	return string(configBytes), nil
}