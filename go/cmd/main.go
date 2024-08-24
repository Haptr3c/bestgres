// main.go

package main

import (
	"fmt"
	"os"

	controller "bestgres/cmd/controller"
	operator "bestgres/cmd/operator"

	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	mode := os.Getenv("MODE")
	switch mode {
	case "operator":
		operator.RunOperator()
	case "controller":
		controller.RunController()
	case "init":
		controller.RunInitController()
	default:
		setupLog.Error(fmt.Errorf("invalid MODE environment variable: %s", mode), "unable to determine operation mode")
		os.Exit(1)
	}
}

