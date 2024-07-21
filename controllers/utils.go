package controllers

import (
	bestgresv1 "bestgres/api/v1"
	"context"
	"crypto/rand"
	"log"
	"math/big"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getOperatorImage() (string) {
    imageName := os.Getenv("OPERATOR_IMAGE")
    if imageName == "" {
        imageName = "bestgres/operator:latest"
    }
    return imageName
}


func generateRandomPassword(length int) (string, error) {
    const charset = "abcdefghijklmnopqrstuvwxyz" +
        "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    password := make([]byte, length)
    for i := range password {
        random, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
        if err != nil {
            return "", err
        }
        password[i] = charset[random.Int64()]
    }
    return string(password), nil
}

func labelsForBGCluster(name string) map[string]string {
	return map[string]string{
		"application":  "spilo",
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

func isOwnedByBGCluster(obj metav1.Object, bgCluster *bestgresv1.BGCluster) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.APIVersion == bgCluster.APIVersion && ref.Kind == bgCluster.Kind && ref.Name == bgCluster.Name {
			return true
		}
	}
	return false
}

func refreshContext(bgCluster *bestgresv1.BGCluster, c client.Client) *bestgresv1.BGCluster {
	err := c.Get(context.TODO(), types.NamespacedName{Name: bgCluster.Name, Namespace: bgCluster.Namespace}, bgCluster)
	if err != nil {
		log.Printf("Error refreshing BGCluster: %v", err)
	}
	return bgCluster
}