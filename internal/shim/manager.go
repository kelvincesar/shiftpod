package shim

import (
	"context"
	"io"

	"github.com/containerd/containerd/api/types"
	runcmanager "github.com/containerd/containerd/v2/cmd/containerd-shim-runc-v2/manager"
	"github.com/containerd/containerd/v2/pkg/shim"
	"github.com/kelvinc/shiftpod/internal"
)

type shiftpodManager struct {
	shimManager shim.Manager
}

func NewShiftpodManager(runtimeName string) shim.Manager {
	return &shiftpodManager{
		shimManager: runcmanager.NewShimManager(runtimeName),
	}
}

// Implements shim.Manager interface
func (m *shiftpodManager) Name() string {
	return m.shimManager.Name()
}

func (m *shiftpodManager) Start(ctx context.Context, id string, opts shim.StartOpts) (shim.BootstrapParams, error) {

	// Starts the shim manager
	params, err := m.shimManager.Start(ctx, id, opts)
	if err != nil {
		internal.Log.Debugf("Start shimmanager failed: %v", err)
		return params, err
	}

	return params, nil
}

func (m *shiftpodManager) Stop(ctx context.Context, id string) (shim.StopStatus, error) {
	internal.Log.Debugf("Stop called for ID: %s", id)
	return m.shimManager.Stop(ctx, id)
}

func (m *shiftpodManager) Info(ctx context.Context, optionsR io.Reader) (*types.RuntimeInfo, error) {
	internal.Log.Debugf("Info called")

	return &types.RuntimeInfo{Name: m.Name()}, nil
}
