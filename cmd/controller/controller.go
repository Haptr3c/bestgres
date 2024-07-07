// controller.go

package controller

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

func RunController() {
	podName, namespace := getPodInfo()
	c := createClient()
	statefulSet, bgCluster := getResources(podName, namespace, c)
	
	runCommand(bgCluster)
	reconciliationLoop(statefulSet, bgCluster, namespace, c)
}

func getPodInfo() (string, string) {
	podName := os.Getenv("POD_NAME")
	namespace := os.Getenv("POD_NAMESPACE")

	if podName == "" || namespace == "" {
		fmt.Println("POD_NAME or POD_NAMESPACE environment variables are not set")
		os.Exit(1)
	}

	return podName, namespace
}

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

func getResources(podName, namespace string, c client.Client) (*appsv1.StatefulSet, *bestgresv1.BGCluster) {
	pod := &corev1.Pod{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: namespace}, pod)
	if err != nil {
		fmt.Printf("Error getting pod: %v\n", err)
		os.Exit(1)
	}

	var statefulSet *appsv1.StatefulSet
	var bgCluster *bestgresv1.BGCluster

	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "StatefulSet" {
			statefulSet = &appsv1.StatefulSet{}
			err = c.Get(context.TODO(), types.NamespacedName{Name: owner.Name, Namespace: namespace}, statefulSet)
			if err != nil {
				fmt.Printf("Error getting StatefulSet: %v\n", err)
				os.Exit(1)
			}

			for _, ssOwner := range statefulSet.OwnerReferences {
				if ssOwner.Kind == "BGCluster" {
					bgCluster = &bestgresv1.BGCluster{}
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
	return nil, nil // This line will never be reached, but it's needed for compilation
}

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

		checkAnnotations(bgCluster, statefulSet, context.TODO(), c)

		time.Sleep(2 * time.Second)
	}
}

func checkAnnotations(bgCluster *bestgresv1.BGCluster, statefulSet *appsv1.StatefulSet, ctx context.Context, c client.Client) {
    if bgCluster.Annotations == nil {
        bgCluster.Annotations = make(map[string]string)
    }

    if value, exists := bgCluster.Annotations[bgClusterInitializedAnnotation]; exists {
		if value == "true" {
			// BGCluster is initialized
			// no-op
		} else {
			fmt.Println("BGCluster not initialized")
		}
	} else {
        fmt.Println("BGCluster not initialized")
        bgCluster.Annotations[bgClusterInitializedAnnotation] = "false"
        if err := c.Update(ctx, bgCluster); err != nil {
            fmt.Printf("Failed to update BGCluster: %v\n", err)
        }
    }

	if value, exists := bgCluster.Annotations[bgDbOpsPendingAnnotation]; exists {
		fmt.Printf("DB Ops pending: %s\n", value)
	}

	if value, exists := bgCluster.Annotations[bgDbOpsOpAnnotation]; exists {
		fmt.Printf("DB Ops operation: %s\n", value)
	}

	if value, exists := statefulSet.Annotations[bgClusterInitializedAnnotation]; exists {
		fmt.Printf("StatefulSet initialized: %s\n", value)
	}
}