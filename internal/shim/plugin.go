package shim

import (
	"fmt"

	runcservice "github.com/containerd/containerd/v2/cmd/containerd-shim-runc-v2/task"
	"github.com/containerd/containerd/v2/pkg/shim"
	"github.com/containerd/containerd/v2/pkg/shutdown"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
)

func init() {
	// Register the TTRPC service with the containerd plugin system
	registry.Register(&plugin.Registration{
		Type: plugins.TTRPCPlugin,
		ID:   "task",
		Requires: []plugin.Type{
			plugins.EventPlugin,
			plugins.InternalPlugin,
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {

			pub, err := ic.GetByID(plugins.EventPlugin, "publisher")
			if err != nil {
				return nil, fmt.Errorf("error collect publisher: %w", err)
			}

			shut, err := ic.GetByID(plugins.InternalPlugin, "shutdown")
			if err != nil {
				return nil, fmt.Errorf("error collect shutdown: %w", err)
			}

			// Create the runc service
			runcSvc, err := runcservice.NewTaskService(ic.Context, pub.(shim.Publisher), shut.(shutdown.Service))
			if err != nil {
				return nil, fmt.Errorf("error collect runc service: %w", err)
			}

			// Wrap with Shiftpod
			shiftpodSvc, err := NewShiftpodService(runcSvc)
			if err != nil {
				return nil, fmt.Errorf("error to create shiftpod wrapper: %w", err)
			}

			ttrpcSvc := &ttrpcTaskWrapper{svc: shiftpodSvc}

			// just to type check before compiling
			var _ shim.TTRPCService = ttrpcSvc
			return ttrpcSvc, nil
		}})
}
