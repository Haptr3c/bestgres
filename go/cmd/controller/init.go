// init.go

package controller

import (
	"fmt"
	"io"
	"os"
)

func RunInitController() {
	// Determine the path to the current executable
	executablePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to get executable path: %v\n", err)
		os.Exit(1)
	}

	// Define the destination path for the binary copy
	destinationPath := "/app/controller"

	// Open the source file
	sourceFile, err := os.Open(executablePath)
	if err != nil {
		fmt.Printf("Failed to open source file: %v\n", err)
		os.Exit(1)
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		fmt.Printf("Failed to create destination file: %v\n", err)
		os.Exit(1)
	}
	defer destinationFile.Close()

	// Copy the file content
	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		fmt.Printf("Failed to copy file: %v\n", err)
		os.Exit(1)
	}

	// Set the file permissions to 555
	if err := os.Chmod(destinationPath, 0555); err != nil {
		fmt.Printf("Failed to set file permissions: %v\n", err)
		os.Exit(1)
	}

	// Set the owner to user 999
	if err := os.Chown(destinationPath, 999, 999); err != nil {
		fmt.Printf("Failed to set file owner: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Binary copied successfully to", destinationPath)
	os.Exit(0)
}
