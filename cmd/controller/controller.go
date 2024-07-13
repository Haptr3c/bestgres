// controller.go

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
)

const (
	bgClusterInitializedAnnotation 	  = "bgcluster.bestgres.io/initialized"
	bgShardedClusterWorkersAnnotation = "bgshardedcluster.bestgres.io/workers"
	bgDbOpsPendingAnnotation          = "bgdbops.bestgres.io/pending"
	bgDbOpsOpAnnotation               = "bgdbops.bestgres.io/op"
	bgClusterRoleLabel        		  = "bgcluster.bestgres.io/role"
	bgClusterPartOfLabel   		      = "bgcluster.bestgres.io/part-of"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

// RunController is the main entry point for the controller
func RunController() {
	podName, namespace := getPodInfo()
	c := createClient()
	bgCluster := getResources(podName, namespace, c)
	
	// first we need to modify the spilo configuration
	hackConfigs(bgCluster)
	// then we run the main container command
	runContainerCommand(bgCluster)

	// wait for the database to be ready
	// otherwise we can't run any SQL commands
	err := waitForDatabase(5 * time.Minute)
	if err != nil {
		log.Printf("Error waiting for database: %v", err)
		os.Exit(1)
	}

	// run the appropriate bootstrap commands based on the BGCluster type
	switch {
	case !isPartOfShardedCluster(bgCluster):
		bootstrapStandaloneBGCluster(bgCluster, c)
	case isWorkerNode(bgCluster):
		bootstrapWorkerNode(bgCluster, c)
	case isCoordinatorNode(bgCluster):
		bootstrapCoordinatorNode(bgCluster, c)
	default:
		log.Println("Cannot determine BGCluster type. Exiting.")
		os.Exit(1)
	}

	reconciliationLoop(bgCluster, c)
}

////////////////////////////////////////
//         Run once functions         //
////////////////////////////////////////

// hackConfigs modifies the Spilo configuration for sharded clusters
func hackConfigs(bgCluster *bestgresv1.BGCluster) {
	filePath := "/scripts/configure_spilo.py"
	input, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading file %s: %v", filePath, err)
		os.Exit(1)
	}
	content := string(input)
	
	if _, exists := bgCluster.Labels[bgClusterPartOfLabel]; exists {
		sharedPreloadConfig := "shared_preload_libraries: 'citus,bg_mon"
		re := regexp.MustCompile(`(?i)shared_preload_libraries:\s*.bg_mon`)
		content = re.ReplaceAllString(content, sharedPreloadConfig)
	} else {
		log.Println("BGCluster is not part of a sharded cluster. No modifications needed.")
	}
	// Replace pg_hba lines with regex
	pgHbaReplacements := []struct {
		oldPattern *regexp.Regexp
		newLine    string
	}{
		{
			oldPattern: regexp.MustCompile(`- host\s+all\s+all\s+127\.0\.0\.1/32\s+md5\s*`),
			newLine:    `- host  all  all  10.0.0.0/8  trust` + "\n"+ `    - host  all  all  127.0.0.1/32  trust`,
		},
		{
			oldPattern: regexp.MustCompile(`- host\s+all\s+all\s+::1/128\s+md5\s*$`),
			newLine:    `- host  all  all  ::1/128  trust`,
		},
	}

	for _, replacement := range pgHbaReplacements {
		content = replacement.oldPattern.ReplaceAllString(content, replacement.newLine)
	}

	// Check if any changes were made to the file
	if content == string(input) {
		log.Println("No changes were made to the file.")
		return
	}

	err = os.WriteFile(filePath, []byte(content), 0755)
	if err != nil {
		log.Printf("Error writing file %s: %v", filePath, err)
		os.Exit(1)
	}
	log.Println("File updated successfully.")
}

// runCommand executes the main container command
func runContainerCommand(bgCluster *bestgresv1.BGCluster) {
	command := bgCluster.Spec.Image.Command
	workingDir := bgCluster.Spec.Image.WorkingDir

	if len(command) == 0 {
		log.Println("No command specified in BGCluster")
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
		log.Printf("Error starting command: %v", err)
		os.Exit(1)
	}

	log.Printf("Started command: %s", strings.Join(command, " "))
}

func bootstrapStandaloneBGCluster(bgCluster *bestgresv1.BGCluster, c client.Client) {
	userBootstrap := bgCluster.Spec.BootstrapSQL
	
	time.Sleep(5 * time.Second)
	// TODO remove the sleep after fixing db readiness check
	// wait for postgres to actually be ready
	log.Println("Running BGCluster bootstrap")
	if err := runPsqlCommands(userBootstrap); err != nil {
		log.Printf("Failed to run bootstrap SQL commands: %v", err)
		// may want to exit here if the bootstrap commands are critical
		// os.Exit(1)
	}
	if err := updateAnnotation(bgCluster, c, bgClusterInitializedAnnotation, "true"); err != nil {
		log.Printf("Bootstrapping complete but annotation not updated for BGCluster: %v", err)
	}
}

func bootstrapWorkerNode(bgCluster *bestgresv1.BGCluster, c client.Client) {
	var systemCommands []string
	userCommands := bgCluster.Spec.BootstrapSQL
	
	time.Sleep(5 * time.Second)
	// TODO remove the sleep after fixing db readiness check
	// wait for postgres to actually be ready
	log.Println("Running worker node bootstrap")

	// Add the Citus extension
	systemCommands = append(systemCommands, "CREATE EXTENSION IF NOT EXISTS citus;")
	// Add any user-defined commands

	// Run the SQL system commands
	if err := runPsqlCommands(systemCommands); err != nil {
		log.Printf("Failed to run system bootstrap SQL commands: %v", err)
		os.Exit(1)
	}
	// Run the SQL user commands
	if err := runPsqlCommands(userCommands); err != nil {
		log.Printf("Failed to run user bootstrap SQL commands: %v", err)
	}
	if err := updateAnnotation(bgCluster, c, bgClusterInitializedAnnotation, "true"); err != nil {
		// TODO add error status to BGCluster
		// log.Printf("Bootstrapping complete but annotation not updated for BGCluster: %v", err)
	}
}

func bootstrapCoordinatorNode(bgCluster *bestgresv1.BGCluster, c client.Client) {
	var systemCommands []string
	userCommands := bgCluster.Spec.BootstrapSQL
	// TODO remove the sleep after fixing db readiness check
	// wait for postgres to actually be ready (patroni readiness check apparently is not good enough)
	time.Sleep(5 * time.Second)
	log.Println("Running coordinator node bootstrap")

	// Add the Citus extension
	systemCommands = append(systemCommands, "CREATE EXTENSION IF NOT EXISTS citus;")
	// Set the coordinator host
	coordinatorHost := bgCluster.Name + "-coordinator"
	systemCommands = append(systemCommands, fmt.Sprintf("SELECT citus_set_coordinator_host('%s', 5432);", coordinatorHost))
	
	// Run the SQL system commands
	if err := runPsqlCommands(systemCommands); err != nil {
		log.Printf("Failed to run system bootstrap SQL commands: %v", err)
		os.Exit(1)
	}
	
	// TODO fix the logic here, right now it waits for workers in sequence
	// it should loop over the workers and add them as they become ready
	// Wait for workers to init and add them to the coordinator
	workerListJSON, exists := bgCluster.Annotations[bgShardedClusterWorkersAnnotation]
	if !exists {
		log.Printf("annotation %s does not exist", bgShardedClusterWorkersAnnotation)
	}
	// Unmarshal the JSON array into a slice of strings
	var workerList []string
	if err := json.Unmarshal([]byte(workerListJSON), &workerList); err != nil {
		log.Printf("failed to unmarshal worker list from annotation: %v", err)
	}

	for _, worker := range workerList {
		log.Printf("List of workers:\n%s", bgCluster.Annotations[bgShardedClusterWorkersAnnotation])
		log.Printf("Adding worker node %s to coordinator", worker)
	
		// calculate the correct annotation to wait for
		annotation := fmt.Sprintf("bgshardedcluster.bestgres.io/%s-initialized", worker)
		// wait for that annotation to bet set to true
		waitForInitilizatedAnnotation(bgCluster, c, 2 * time.Minute, annotation)
		// calculate the sql command to add the worker to the coordinator
		command := fmt.Sprintf("SELECT * FROM citus_add_node('%s', 5432);", worker)
		// run the command
		if err := runPsqlCommand(command, 5, 5 * time.Second); err != nil {
			log.Printf("Failed to add worker node %s to coordinator: %v", worker, err)
		}
	}
	
	// Run the SQL user commands
	if err := runPsqlCommands(userCommands); err != nil {
		log.Printf("Failed to run user bootstrap SQL commands: %v", err)
	}
	if err := updateAnnotation(bgCluster, c, bgClusterInitializedAnnotation, "true"); err != nil {
		// TODO add error status to BGCluster
		// log.Printf("Bootstrapping complete but annotation not updated for BGCluster: %v", err)
	}
}

////////////////////////////////////////
//           Loop functions           //
////////////////////////////////////////

// reconciliationLoop continuously checks for updates and performs reconciliation
func reconciliationLoop(bgCluster *bestgresv1.BGCluster, c client.Client) {
	for {
		refreshContext(bgCluster, c)
		time.Sleep(2 * time.Second)
		for annotation := range bgCluster.Annotations {
			value := checkAnnotation(bgCluster, annotation)
			if annotation == bgDbOpsPendingAnnotation {
				if value == "true" {
					// TODO handle BGDbOps
				// 	if err := handleBgDbOps(bgCluster); err != nil {
				// 		log.Printf("Failed to handle BGDbOps: %v", err)
				// }
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
}

// checkAnnotations checks for annotations and returns the value
func checkAnnotation(bgCluster *bestgresv1.BGCluster, annotation string) string {
	return bgCluster.Annotations[annotation]
}

////////////////////////////////////////
//          Helper functions          //
////////////////////////////////////////

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

// getResources retrieves the StatefulSet and BGCluster resources
func getResources(podName, namespace string, c client.Client) *bestgresv1.BGCluster {
	pod := &corev1.Pod{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: namespace}, pod)
	if err != nil {
		log.Printf("Error getting pod: %v", err)
		os.Exit(1)
	}

	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "StatefulSet" {
			statefulSet := &appsv1.StatefulSet{}
			err = c.Get(context.TODO(), types.NamespacedName{Name: owner.Name, Namespace: namespace}, statefulSet)
			if err != nil {
				log.Printf("Error getting StatefulSet: %v", err)
				os.Exit(1)
			}

			for _, ssOwner := range statefulSet.OwnerReferences {
				if ssOwner.Kind == "BGCluster" {
					bgCluster := &bestgresv1.BGCluster{}
					err = c.Get(context.TODO(), types.NamespacedName{Name: ssOwner.Name, Namespace: namespace}, bgCluster)
					if err != nil {
						log.Printf("Error getting BGCluster: %v", err)
						os.Exit(1)
					}
					return bgCluster
				}
			}
			log.Printf("Could not find owning BGCluster for StatefulSet %s", statefulSet.Name)
		}
	}
	log.Printf("Could not find owning StatefulSet for Pod %s", podName)
	return nil
}

// getPodInfo retrieves the pod name and namespace from environment variables
func getPodInfo() (string, string) {
	podName := os.Getenv("POD_NAME")
	namespace := os.Getenv("POD_NAMESPACE")

	if podName == "" || namespace == "" {
		log.Println("POD_NAME or POD_NAMESPACE environment variables are not set")
		os.Exit(1)
	}

	return podName, namespace
}

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

func updateAnnotation(bgCluster *bestgresv1.BGCluster, c client.Client, key, value string) error {
	if bgCluster.Annotations == nil {
		bgCluster.Annotations = make(map[string]string)
	}
	bgCluster.Annotations[key] = value
	if err := c.Update(context.TODO(), bgCluster); err != nil {
		log.Printf("Failed to update BGCluster: %v", err)
		return err
	}
	return nil
}

func refreshContext(bgCluster *bestgresv1.BGCluster, c client.Client) *bestgresv1.BGCluster {
	err := c.Get(context.TODO(), types.NamespacedName{Name: bgCluster.Name, Namespace: bgCluster.Namespace}, bgCluster)
	if err != nil {
		log.Printf("Error refreshing BGCluster: %v", err)
	}
	return bgCluster
}

func isPartOfShardedCluster(bgCluster *bestgresv1.BGCluster) bool {
	_, exists := bgCluster.Labels[bgClusterPartOfLabel]
	return exists
}

func isCoordinatorNode(bgCluster *bestgresv1.BGCluster) bool {
	return bgCluster.Labels[bgClusterRoleLabel] == "coordinator"
}

func isWorkerNode(bgCluster *bestgresv1.BGCluster) bool {
	return bgCluster.Labels[bgClusterRoleLabel] == "worker"
}