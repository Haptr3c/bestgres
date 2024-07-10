package controllers

import (
	"context"
	"fmt"

	bestgresv1 "bestgres/api/v1"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
		cm := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: bgCluster.Namespace}, cm)

		if err != nil {
			if errors.IsNotFound(err) {
				// ConfigMap doesn't exist yet, which is fine
				log.Info(fmt.Sprintf("ConfigMap %s not found, skipping", cmName))
				continue
			}
			return err
		}

		// Check if the BGCluster is already the owner
		if isOwnedByBGCluster(cm, bgCluster) {
			log.Info(fmt.Sprintf("ConfigMap %s is already owned by BGCluster, skipping", cmName))
			continue
		}

		// Update the owner reference in local object
		if err := ctrl.SetControllerReference(bgCluster, cm, r.Scheme); err != nil {
			log.Error(err, fmt.Sprintf("Failed to set controller reference for ConfigMap %s", cmName))
			return err
		}

		// Update the ConfigMap in the API
		if err := r.Update(ctx, cm); err != nil {
			log.Error(err, fmt.Sprintf("Failed to update ConfigMap %s", cmName))
			return err
		}

		log.Info(fmt.Sprintf("Successfully updated owner reference for ConfigMap %s", cmName))
	}

	return nil
}

func (r *BGClusterReconciler) reconcileSpiloConfigMap(ctx context.Context, bgCluster *bestgresv1.BGCluster) error {
	log := ctrl.LoggerFrom(ctx)
	var configMapKey = "postgres.yaml"

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

	foundCm := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, foundCm)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Creating ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
			return r.Create(ctx, cm)
		}
		return err
	}

	if foundCm.Data[configMapKey] != cm.Data[configMapKey] {
		log.Info("Updating ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
		foundCm.Data = cm.Data
		return r.Update(ctx, foundCm)
	}

	return nil
}

func (r *BGClusterReconciler) createSpiloConfiguration() (string, error) {
	baseConfig := map[string]interface{}{
		"bootstrap": map[string]interface{}{
			"initdb": []map[string]string{
				{"auth-host": "md5"},
				{"auth-local": "trust"},
			},
			"dcs": map[string]interface{}{
				"retry_timeout": 10000, // TODO Test removing this or changing it to like 30 or something
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