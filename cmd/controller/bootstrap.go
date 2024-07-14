package controller

import (
	bestgresv1 "bestgres/api/v1"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
	// If this bgcluster has already been initialized, we assume this pod is coming back up
	// after a restart. In that case, we don't need to run the bootstrap commands again.
	if checkAnnotation(bgCluster, bgClusterInitializedAnnotation) == "true" {
		// If there is a pending operation to restart the cluster, we need to mark it complete
		if checkAnnotation(bgCluster, bgDbOpsPendingAnnotation) == "true" && checkAnnotation(bgCluster, bgDbOpsOpAnnotation) == "restart" {
			log.Println("Marking restart operation as complete")
			updateAnnotation(c, podName, namespace, bgDbOpsCompletedAnnotation, "true")
		}
	} else {
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

	if err := updateAnnotation(c, podName, namespace, bgClusterInitializedAnnotation, "true"); err != nil {
		log.Printf("Bootstrapping complete but annotation not updated for Pod: %v", err)
	}
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

    if err := updateAnnotation(c, podName, namespace, bgClusterInitializedAnnotation, "true"); err != nil {
        log.Printf("Bootstrapping complete but annotation not updated for Pod: %v", err)
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
    if err := updateAnnotation(c, podName, namespace, bgClusterInitializedAnnotation, "true"); err != nil {
        log.Printf("Bootstrapping complete but annotation not updated for Pod: %v", err)
    }
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