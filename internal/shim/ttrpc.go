package shim

import (
	taskAPI "github.com/containerd/containerd/api/runtime/task/v3"
	"github.com/containerd/log"
	"github.com/containerd/ttrpc"
)

// This wrapper is used to register the TTRPC service with the containerd shim
// Will be called by the registration process in the plugin
type ttrpcTaskWrapper struct {
	svc taskAPI.TTRPCTaskService
}

// NewTTRPCWrapper register the TTRPC server
func (w *ttrpcTaskWrapper) RegisterTTRPC(server *ttrpc.Server) error {
	defer func() {
		if r := recover(); r != nil {
			log.L.WithField("component", "shiftpod").Errorf("panic in RegisterTTRPC: %v", r)
		}
	}()

	taskAPI.RegisterTTRPCTaskService(server, w.svc)
	return nil
}
