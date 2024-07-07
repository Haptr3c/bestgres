package controllers

import (
	"crypto/rand"
	"math/big"
	"os"

	corev1 "k8s.io/api/core/v1"
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