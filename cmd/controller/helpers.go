package controller

import (
	bestgresv1 "bestgres/api/v1"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// createClient creates a new Kubernetes client
func createClient() client.Client {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Printf("Error getting config: %v", err)
		os.Exit(1)
	}

	err = bestgresv1.AddToScheme(scheme.Scheme)
	if err != nil {
		log.Printf("Error adding bestgres scheme: %v", err)
		os.Exit(1)
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Printf("Error creating client: %v", err)
		os.Exit(1)
	}

	return c
}

func refreshContext(bgCluster *bestgresv1.BGCluster, c client.Client) *bestgresv1.BGCluster {
	err := c.Get(context.TODO(), types.NamespacedName{Name: bgCluster.Name, Namespace: bgCluster.Namespace}, bgCluster)
	if err != nil {
		log.Printf("Error refreshing BGCluster: %v", err)
	}
	return bgCluster
}

// checkAnnotation checks for annotations and returns the value
func checkAnnotation(bgCluster *bestgresv1.BGCluster, annotation string) string {
	return bgCluster.Annotations[annotation]
}

func checkPodAnnotation(c client.Client, podName string, namespace string, annotation string) string {
	pod := &corev1.Pod{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: namespace}, pod)
	if err != nil {
		log.Printf("Error getting pod: %v", err)
	}
	return pod.Annotations[annotation]
}

func deletePod(c client.Client, podName string, namespace string) {
	pod := &corev1.Pod{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: namespace}, pod)
	if err != nil {
		log.Printf("Error getting pod: %v", err)
	}
	err = c.Delete(context.TODO(), pod)
	if err != nil {
		log.Printf("Error deleting pod: %v", err)
	}
}

func updateAnnotation(c client.Client, podName, namespace, key, value string) error {
    pod := &corev1.Pod{}
    err := c.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: namespace}, pod)
    if err != nil {
        return fmt.Errorf("failed to get pod: %v", err)
    }

    if pod.Annotations == nil {
        pod.Annotations = make(map[string]string)
    }
    pod.Annotations[key] = value

    if err := c.Update(context.TODO(), pod); err != nil {
        return fmt.Errorf("failed to update pod: %v", err)
    }
    return nil
}

func deleteAnnotation(c client.Client, podName, namespace, key string) error {
	pod := &corev1.Pod{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: namespace}, pod)
	if err != nil {
		return fmt.Errorf("failed to get pod: %v", err)
	}

	delete(pod.Annotations, key)

	if err := c.Update(context.TODO(), pod); err != nil {
		return fmt.Errorf("failed to update pod: %v", err)
	}
	return nil
}

// getResources retrieves the BGCluster resource using the cluster-name label on the pod
func getResources(podName, namespace string, c client.Client) *bestgresv1.BGCluster {
	// Retrieve the pod
	pod := &corev1.Pod{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: namespace}, pod)
	if err != nil {
		log.Printf("Error getting pod: %v", err)
		os.Exit(1)
	}

	// Extract the cluster-name label from the pod
	clusterName, exists := pod.Labels["cluster-name"]
	if !exists {
		log.Printf("cluster-name label not found on Pod %s", podName)
		os.Exit(1)
	}

	// Retrieve the BGCluster resource using the cluster-name label
	bgCluster := &bestgresv1.BGCluster{}
	err = c.Get(context.TODO(), types.NamespacedName{Name: clusterName, Namespace: namespace}, bgCluster)
	if err != nil {
		log.Printf("Error getting BGCluster: %v", err)
		os.Exit(1)
	}

	return bgCluster
}

// runAllPsqlCommands executes all SQL commands with error handling and retries
func runPsqlCommands(commands []string) error {
	maxRetries := 5
	retryInterval := 5 * time.Second

	for _, command := range commands {
		log.Printf("Executing command: %s", command)
		if err := runPsqlCommand(command, maxRetries, retryInterval); err != nil {
			log.Printf("Failed to execute command '%s': %v", command, err)
			return err
		}
	}
	return nil
}

// runPsqlCommand executes a single SQL command with retries
func runPsqlCommand(sqlCommand string, maxRetries int, retryInterval time.Duration) error {
	var stdout, stderr bytes.Buffer

	for attempt := 0; attempt < maxRetries; attempt++ {
		cmd := exec.Command("psql", "-U", "postgres", "-c", sqlCommand)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			// Command succeeded
			return nil
		}

		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			// This is not an ExitError, so it's likely a more severe issue
			return fmt.Errorf("failed to execute psql command: %v", err)
		}

		if exitErr.ExitCode() == 1 {
			// Exit code 1 usually means a SQL error, which we don't want to retry
			return fmt.Errorf("SQL error: %s", stderr.String())
		}

		log.Printf("Command failed (attempt %d/%d): %s\nRetrying in %v...", attempt+1, maxRetries, stderr.String(), retryInterval)
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("failed to execute psql command after %d attempts: %s", maxRetries, stderr.String())
}

// runCommand executes a single linux command with retries
func runCommand(command string, maxRetries int, retryInterval time.Duration) error {
	var stdout, stderr bytes.Buffer

	for attempt := 0; attempt < maxRetries; attempt++ {
		cmd := exec.Command(command)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			// Command succeeded
			return nil
		}

		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			// This is not an ExitError, so it's likely a more severe issue
			return fmt.Errorf("failed to execute command: %v", err)
		}

		if exitErr.ExitCode() == 1 {
			// Exit code 1 usually means a SQL error, which we don't want to retry
			return fmt.Errorf("Error: %s", stderr.String())
		}

		log.Printf("Command failed (attempt %d/%d): %s\nRetrying in %v...", attempt+1, maxRetries, stderr.String(), retryInterval)
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("failed to execute command after %d attempts: %s", maxRetries, stderr.String())
}

func waitForInitilizatedAnnotation(bgCluster *bestgresv1.BGCluster, c client.Client, timeout time.Duration, annotation string) error {
	start := time.Now()
	for {
		refreshContext(bgCluster, c)
		ready := checkAnnotation(bgCluster, annotation)
		if ready == "true" {
			log.Println("Database is ready for connections.")
			return nil
		} else {
			log.Printf("Waiting for %s", annotation)
			time.Sleep(5 * time.Second)
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for database to be ready")
		}
		time.Sleep(2 * time.Second)
		log.Println("Waiting for database to be ready...")
		continue
	}
}

// waitForDatabase waits for the database to be ready
func waitForDatabase(timeout time.Duration) error {
	start := time.Now()
	for {
		ready, err := checkPatroniStatus()
		if ready {
			log.Println("Database is ready for connections.")
			return nil
		}
		if err != nil {
			time.Sleep(5 * time.Second)
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for database to be ready")
		}
		time.Sleep(2 * time.Second)
		log.Println("Waiting for database to be ready...")
		continue
	}
}

// PatroniStatus represents the structure of the JSON response from the Patroni API
type PatroniStatus struct {
	State string `json:"state"`
}

// checkPatroniStatus checks if the Patroni cluster is in a running state
func checkPatroniStatus() (bool, error) {
	url := "http://localhost:8008/patroni"
	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var status PatroniStatus
	err = json.Unmarshal(body, &status)
	if err != nil {
		return false, err
	}

	return status.State == "running", nil
}
