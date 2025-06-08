package main

import (
	"context"

	"github.com/containerd/containerd/v2/pkg/shim"
	"github.com/containerd/log"
	"github.com/kelvinc/shiftpod/internal"
	shiftpodshim "github.com/kelvinc/shiftpod/internal/shim"
)

// main is the entry point for the Shiftpod shim
// Do not print any logs before shi.Run()
func main() {
	internal.SetupLogger()

	managerInstance := shiftpodshim.NewShiftpodManager(internal.RUNTIME_NAME)
	if managerInstance == nil {
		logger := log.L.WithField("component", "shiftpod")
		logger.Fatal("Failed to create shiftpod manager instance (returned nil)")
		return
	}

	shim.Run(context.Background(), managerInstance)
}
