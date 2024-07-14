// controller.go

package controller

import (
	bestgresv1 "bestgres/api/v1"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	bgClusterRoleLabel        		  = "bgcluster.bestgres.io/role"
	bgClusterPartOfLabel   		      = "bgcluster.bestgres.io/part-of"
	bgClusterInitializedAnnotation 	  = "bgcluster.bestgres.io/initialized"
	bgShardedClusterWorkersAnnotation = "bgshardedcluster.bestgres.io/workers"
	bgDbOpsPendingAnnotation          = "bgdbops.bestgres.io/pending"
	bgDbOpsOpAnnotation               = "bgdbops.bestgres.io/op"
	bgDbOpsSpecAnnotation             = "bgdbops.bestgres.io/spec"
	bgDbOpsCompletedAnnotation        = "bgdbops.bestgres.io/completed"
	bgDbOpsInProgressAnnotation       = "bgdbops.bestgres.io/in-progress"
)

var podName = os.Getenv("POD_NAME")
var namespace = os.Getenv("POD_NAMESPACE")

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

// RunController is the main entry point for the controller
func RunController() {
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

// reconciliationLoop continuously checks for updates and performs reconciliation
func reconciliationLoop(bgCluster *bestgresv1.BGCluster, c client.Client) {
    for {
        // Refresh the BGCluster object
        bgCluster := refreshContext(bgCluster, c)

        // Check if there's a pending operation
        if bgCluster.Annotations[bgDbOpsPendingAnnotation] == "true" {
            if err := handleBgDbOps(bgCluster, c); err != nil {
                log.Printf("Failed to handle BGDbOps: %v", err)
            }
        } else {
            // make sure to remove any old completed annotations
            deleteAnnotation(c, podName, namespace, bgDbOpsCompletedAnnotation)
        }
        // TODO remove this sleep (not sure why this is here tbh)
        time.Sleep(2 * time.Second)
    }
}

func handleBgDbOps(bgCluster *bestgresv1.BGCluster, c client.Client) error {
    op := bgCluster.Annotations[bgDbOpsOpAnnotation]
    spec := bgCluster.Annotations[bgDbOpsSpecAnnotation]

    var err error
    switch op {
    case "restart":
        err = handleRestart(c, bgCluster, spec)
    case "backup":
        err = handleBackup(c, bgCluster, spec)
    case "benchmark":
        err = handleBenchmark(c, bgCluster, spec)
    case "repack":
        err = handleRepack(c, bgCluster, spec)
    case "vacuum":
        err = handleVacuum(c, bgCluster, spec)
    default:
        return fmt.Errorf("unknown operation: %s", op)
    }

    if err != nil {
        return err
    }

    // Set the BGDbOps completed annotation
    return updateAnnotation(c, podName, namespace, bgDbOpsCompletedAnnotation, "true")
}

func handleRestart(c client.Client, bgCluster *bestgresv1.BGCluster, spec string) error {
	// TODO also need to check if this is a replica or master and handle accordingly
    if checkPodAnnotation(c, podName, namespace, bgDbOpsCompletedAnnotation) != "true" {
        log.Printf("Handling restart operation for %s", bgCluster.Name)
        runCommand("sv stop patroni", 0, 1*time.Second)
        deletePod(c, podName, namespace)
    } else {
        // update the bgdbop object to indicate that the operation is complete
        // we have to get the name of the bgdbop object from the bgcluster object's in-progress annotation
        bgDbOpsName := bgCluster.Annotations[bgDbOpsInProgressAnnotation]
        bgDbOps := &bestgresv1.BGDbOps{}
        err := c.Get(context.TODO(), types.NamespacedName{Name: bgDbOpsName, Namespace: namespace}, bgDbOps)
        if err != nil {
            return fmt.Errorf("failed to get BGDbOps: %v", err)
        }
        // create an annotation on the bgdbop to indicate that the operation is complete
        podCompletedAnnotation := string("bgcluster.bestgres.io/" + podName )
        bgDbOps.Annotations[podCompletedAnnotation] = "true"
        if err := c.Update(context.TODO(), bgDbOps); err != nil {
            return fmt.Errorf("failed to update BGDbOps: %v", err)
        }
    }
    return nil
}

func handleBackup(c client.Client, bgCluster *bestgresv1.BGCluster, spec string) error {
    // Placeholder function for backup operation
    log.Printf("Handling backup operation for %s", bgCluster.Name)
    return nil
}

func handleBenchmark(c client.Client, bgCluster *bestgresv1.BGCluster, spec string) error {
    // Placeholder function for benchmark operation
    log.Printf("Handling benchmark operation for %s", bgCluster.Name)
    return nil
}

func handleRepack(c client.Client, bgCluster *bestgresv1.BGCluster, spec string) error {
    // Placeholder function for repack operation
    log.Printf("Handling repack operation for %s", bgCluster.Name)
    return nil
}

func handleVacuum(c client.Client, bgCluster *bestgresv1.BGCluster, spec string) error {
    // Placeholder function for vacuum operation
    log.Printf("Handling vacuum operation for %s", bgCluster.Name)
    return nil
}
