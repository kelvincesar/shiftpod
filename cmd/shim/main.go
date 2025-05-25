package main

import (
	"context"
	"time"

	"github.com/containerd/containerd/v2/pkg/shim"
	"github.com/containerd/log"
	"github.com/kelvinc/shiftpod/internal"
	"github.com/sirupsen/logrus"

	shiftpodshim "github.com/kelvinc/shiftpod/internal/shim"
)

func main() {
	log.L.Logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
	})

	managerInstance := shiftpodshim.NewShiftpodManager(internal.RUNTIME_NAME)
	if managerInstance == nil {
		logger := log.L.WithField("component", "shiftpod")
		logger.Fatal("Failed to create shiftpod manager instance (returned nil)")
		return
	}

	shim.Run(context.Background(), managerInstance)
}
