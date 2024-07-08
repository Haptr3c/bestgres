// controller.go

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	bestgresv1 "bestgres/api/v1"
)

const (
	bgClusterInitializedAnnotation = "bgcluster.bestgres.io/initialized"
	bgDbOpsPendingAnnotation       = "bgdbops.bestgres.io/pending"
	bgDbOpsOpAnnotation            = "bgdbops.bestgres.io/op"
)

// RunController is the main entry point for the controller
func RunController() {
	podName, namespace := getPodInfo()
	c := createClient()
	statefulSet, bgCluster := getResources(podName, namespace, c)
	
	hackConfigs(bgCluster)
	runCommand(bgCluster)
	err := waitForDatabase(5 * time.Minute)
	if err != nil {
		fmt.Printf("Error waiting for database: %v\n", err)
		return
	}
	reconciliationLoop(statefulSet, bgCluster, namespace, c)
}

// getPodInfo retrieves the pod name and namespace from environment variables
func getPodInfo() (string, string) {
	podName := os.Getenv("POD_NAME")
	namespace := os.Getenv("POD_NAMESPACE")

	if podName == "" || namespace == "" {
		fmt.Println("POD_NAME or POD_NAMESPACE environment variables are not set")
		os.Exit(1)
	}

	return podName, namespace
}

// createClient creates a new Kubernetes client
func createClient() client.Client {
	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Printf("Error getting config: %v\n", err)
		os.Exit(1)
	}

	err = bestgresv1.AddToScheme(scheme.Scheme)
	if err != nil {
		fmt.Printf("Error adding bestgres scheme: %v\n", err)
		os.Exit(1)
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	return c
}

// getResources retrieves the StatefulSet and BGCluster resources
func getResources(podName, namespace string, c client.Client) (*appsv1.StatefulSet, *bestgresv1.BGCluster) {
	pod := &corev1.Pod{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: namespace}, pod)
	if err != nil {
		fmt.Printf("Error getting pod: %v\n", err)
		os.Exit(1)
	}

	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "StatefulSet" {
			statefulSet := &appsv1.StatefulSet{}
			err = c.Get(context.TODO(), types.NamespacedName{Name: owner.Name, Namespace: namespace}, statefulSet)
			if err != nil {
				fmt.Printf("Error getting StatefulSet: %v\n", err)
				os.Exit(1)
			}

			for _, ssOwner := range statefulSet.OwnerReferences {
				if ssOwner.Kind == "BGCluster" {
					bgCluster := &bestgresv1.BGCluster{}
					err = c.Get(context.TODO(), types.NamespacedName{Name: ssOwner.Name, Namespace: namespace}, bgCluster)
					if err != nil {
						fmt.Printf("Error getting BGCluster: %v\n", err)
						os.Exit(1)
					}
					return statefulSet, bgCluster
				}
			}
		}
	}

	fmt.Println("Could not find owning StatefulSet or BGCluster")
	os.Exit(1)
	return nil, nil
}

// hackConfigs modifies the Spilo configuration for sharded clusters
func hackConfigs(bgCluster *bestgresv1.BGCluster) {
	sharedPreloadConfig := "shared_preload_libraries: 'citus,bg_mon"

	if _, exists := bgCluster.Labels["bestgres.io/part-of"]; exists {
		filePath := "/scripts/configure_spilo.py"
		input, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", filePath, err)
			os.Exit(1)
		}

		content := string(input)

		re := regexp.MustCompile(`(?i)shared_preload_libraries:\s*.bg_mon`)
		content = re.ReplaceAllString(content, sharedPreloadConfig)

		// Replace pg_hba lines with regex
		pgHbaReplacements := []struct {
			oldPattern *regexp.Regexp
			newLine    string
		}{
			{
				oldPattern: regexp.MustCompile(`(?m)\s*- host\s+all\s+all\s+127\.0\.0\.1/32\s+md5\s*`),
				newLine:    `- host  all  all  10.0.0.0/8  trust` + "\n" + `  - host  all  all 127.0.0.1 trust`,
			},
			{
				oldPattern: regexp.MustCompile(`(?m)\s*- host\s+all\s+all\s+::1/128\s+md5\s*$`),
				newLine:    `- host  all  all  ::1/128  trust`,
			},
		}

		for _, replacement := range pgHbaReplacements {
			content = replacement.oldPattern.ReplaceAllString(content, replacement.newLine)
		}

		// Check if any changes were made to the file
		if content == string(input) {
			fmt.Println("No changes were made to the file.")
			return
		}

		err = os.WriteFile(filePath, []byte(content), 0755)
		if err != nil {
			fmt.Printf("Error writing file %s: %v\n", filePath, err)
			os.Exit(1)
		}
		fmt.Println("File updated successfully.")
	} else {
		fmt.Println("BGCluster is not part of a sharded cluster. No modifications needed.")
	}
}

// runCommand executes the main container command
func runCommand(bgCluster *bestgresv1.BGCluster) {
	command := bgCluster.Spec.Image.Command
	workingDir := bgCluster.Spec.Image.WorkingDir

	if len(command) == 0 {
		fmt.Println("No command specified in BGCluster")
		os.Exit(1)
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting command: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Started command: %s\n", strings.Join(command, " "))
}

// reconciliationLoop continuously checks for updates and performs reconciliation
func reconciliationLoop(statefulSet *appsv1.StatefulSet, bgCluster *bestgresv1.BGCluster, namespace string, c client.Client) {
	for {
		err := c.Get(context.TODO(), types.NamespacedName{Name: bgCluster.Name, Namespace: namespace}, bgCluster)
		if err != nil {
			fmt.Printf("Error refreshing BGCluster: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		err = c.Get(context.TODO(), types.NamespacedName{Name: statefulSet.Name, Namespace: namespace}, statefulSet)
		if err != nil {
			fmt.Printf("Error refreshing StatefulSet: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		checkAnnotations(bgCluster, statefulSet, c)

		time.Sleep(2 * time.Second)
	}
}

// checkAnnotations processes annotations and performs necessary actions
func checkAnnotations(bgCluster *bestgresv1.BGCluster, statefulSet *appsv1.StatefulSet, c client.Client) {
	ctx := context.TODO()

	if bgCluster.Annotations == nil {
		bgCluster.Annotations = make(map[string]string)
	}

	if value, exists := bgCluster.Annotations[bgClusterInitializedAnnotation]; exists {
		userBootstrap := bgCluster.Spec.BootstrapSQL
		if value != "true" {
			fmt.Println("BGCluster not initialized")
			runAllPsqlCommands(bgCluster, userBootstrap, c)
			bgCluster.Annotations[bgClusterInitializedAnnotation] = "true"
			if err := c.Update(ctx, bgCluster); err != nil {
				fmt.Printf("Failed to update BGCluster: %v\n", err)
			}
		}
	} else {
		fmt.Println("BGCluster not initialized")
		bgCluster.Annotations[bgClusterInitializedAnnotation] = "false"
		if err := c.Update(ctx, bgCluster); err != nil {
			fmt.Printf("Failed to update BGCluster: %v\n", err)
		}
	}

	// Log other annotations
	logAnnotation(bgCluster.Annotations, bgDbOpsPendingAnnotation, "DB Ops pending")
	logAnnotation(bgCluster.Annotations, bgDbOpsOpAnnotation, "DB Ops operation")
	logAnnotation(statefulSet.Annotations, bgClusterInitializedAnnotation, "StatefulSet initialized")
}

// logAnnotation is a helper function to log annotation values
func logAnnotation(annotations map[string]string, key, description string) {
	if value, exists := annotations[key]; exists {
		fmt.Printf("%s: %s\n", description, value)
	}
}

// waitForDatabase waits for the database to be ready
func waitForDatabase(timeout time.Duration) error {
	start := time.Now()
	for {
		ready, err := checkPatroniStatus()
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for database to be ready")
		}
		time.Sleep(5 * time.Second)
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

// runPsqlCommand executes a single SQL command
func runPsqlCommand(sqlCommand string) error {
	cmd := exec.Command("psql", "-U", "postgres", "-c", sqlCommand)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute psql command: %v", err)
	}
	return nil
}

// runAllPsqlCommands executes all bootstrap SQL commands
func runAllPsqlCommands(bgCluster *bestgresv1.BGCluster, userCommands []string, c client.Client) error {
	allCommands := getBootstrapSQL(bgCluster, userCommands, c)
	for _, command := range allCommands {
		fmt.Printf("Executing command: %s\n", command)
		err := runPsqlCommand(command)
		if err != nil {
			return err
		}
	}
	fmt.Println("Successfully executed bootstrap SQL commands")
	return nil
}

// getBootstrapSQL generates the bootstrap SQL commands based on the cluster role
func getBootstrapSQL(bgCluster *bestgresv1.BGCluster, userCommands []string, c client.Client) []string {
	var systemCommands []string

	switch bgCluster.Labels["bestgres.io/role"] {
	case "worker":
		systemCommands = append(systemCommands, "CREATE EXTENSION IF NOT EXISTS citus;")
	case "coordinator":
		bgShardedCluster := getBGShardedCluster(*bgCluster, c)
		coordinatorHost := bgCluster.Name + "-coordinator"
		systemCommands = append(systemCommands,
			"CREATE EXTENSION IF NOT EXISTS citus;",
			fmt.Sprintf("SELECT citus_set_coordinator_host('%s', 5432);", coordinatorHost),
		)
		for i := range bgShardedCluster.Spec.Shards {
			workerHost := fmt.Sprintf("%s-worker-%d", bgCluster.Name, i)
			systemCommands = append(systemCommands, fmt.Sprintf("SELECT * FROM citus_add_node('%s', 5432);", workerHost))
		}
	}

	return append(systemCommands, userCommands...)
}

// getBGShardedCluster retrieves the BGShardedCluster resource
func getBGShardedCluster(bgCluster bestgresv1.BGCluster, c client.Client) *bestgresv1.BGShardedCluster {
	for _, bgClusterOwner := range bgCluster.OwnerReferences {
		if bgClusterOwner.Kind == "BGShardedCluster" {
			bgShardedCluster := &bestgresv1.BGShardedCluster{}
			err := c.Get(context.TODO(), types.NamespacedName{Name: bgClusterOwner.Name, Namespace: bgCluster.Namespace}, bgShardedCluster)
			if err != nil {
				fmt.Printf("Error getting BGShardedCluster: %v\n", err)
				os.Exit(1)
			}
			return bgShardedCluster
		}
	}
	fmt.Println("Could not find owning BGShardedCluster")
	os.Exit(1)
	return nil
}