package shim

import (
	"context"
	"fmt"

	taskAPI "github.com/containerd/containerd/api/runtime/task/v3"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/kelvinc/shiftpod/internal"
)

type shiftpodService struct {
	runcService taskAPI.TTRPCTaskService
}

// Builder for the wrapper
func NewShiftpodService(runcService taskAPI.TTRPCTaskService) (taskAPI.TTRPCTaskService, error) {
	if runcService == nil {
		log.L.Error("Cannot initialize: underlying runc service is nil")
		return nil, fmt.Errorf("underlying runc service cannot be nil")
	}
	log.L.Info("Shiftpod wrapper initialized successfully")
	return &shiftpodService{
		runcService: runcService,
	}, nil
}

func (s *shiftpodService) Create(ctx context.Context, r *taskAPI.CreateTaskRequest) (*taskAPI.CreateTaskResponse, error) {

	internal.Log.Infof("Create called: ID=%s, Bundle=%s", r.ID, r.Bundle)
	resp, err := s.runcService.Create(ctx, r)
	if err != nil {
		if errdefs.IsNotImplemented(err) {
			internal.Log.Info("Create not implemented by underlying shim")
		} else {
			internal.Log.Errorf("Create failed: %v", err)
		}
	}
	return resp, err
}

func (s *shiftpodService) Start(ctx context.Context, r *taskAPI.StartRequest) (*taskAPI.StartResponse, error) {
	internal.Log.Infof("Start called: ID=%s, ExecID=%s", r.ID, r.ExecID)
	return s.runcService.Start(ctx, r)
}

func (s *shiftpodService) Delete(ctx context.Context, r *taskAPI.DeleteRequest) (*taskAPI.DeleteResponse, error) {
	internal.Log.Infof("Delete called: ID=%s, ExecID=%s", r.ID, r.ExecID)
	return s.runcService.Delete(ctx, r)
}

func (s *shiftpodService) Pause(ctx context.Context, r *taskAPI.PauseRequest) (*ptypes.Empty, error) {
	internal.Log.Debugf("Pause: ID=%s", r.ID)
	return s.runcService.Pause(ctx, r)
}

func (s *shiftpodService) Resume(ctx context.Context, r *taskAPI.ResumeRequest) (*ptypes.Empty, error) {
	internal.Log.Debugf("Resume: ID=%s", r.ID)
	return s.runcService.Resume(ctx, r)
}

func (s *shiftpodService) State(ctx context.Context, r *taskAPI.StateRequest) (*taskAPI.StateResponse, error) {
	internal.Log.Debugf("State: ID=%s, ExecID=%s", r.ID, r.ExecID)
	return s.runcService.State(ctx, r)
}

func (s *shiftpodService) Pids(ctx context.Context, r *taskAPI.PidsRequest) (*taskAPI.PidsResponse, error) {
	internal.Log.Debugf("Pids: ID=%s", r.ID)
	return s.runcService.Pids(ctx, r)
}

func (s *shiftpodService) Checkpoint(ctx context.Context, r *taskAPI.CheckpointTaskRequest) (*ptypes.Empty, error) {
	internal.Log.Debugf("Checkpoint: ID=%s, Path=%s", r.ID, r.Path)
	return s.runcService.Checkpoint(ctx, r)
}

func (s *shiftpodService) Kill(ctx context.Context, r *taskAPI.KillRequest) (*ptypes.Empty, error) {
	internal.Log.Debugf("Kill: ID=%s, ExecID=%s", r.ID, r.ExecID)
	return s.runcService.Kill(ctx, r)
}

func (s *shiftpodService) Exec(ctx context.Context, r *taskAPI.ExecProcessRequest) (*ptypes.Empty, error) {
	internal.Log.Debugf("Exec: ID=%s, ExecID=%s", r.ID, r.ExecID)
	return s.runcService.Exec(ctx, r)
}

func (s *shiftpodService) ResizePty(ctx context.Context, r *taskAPI.ResizePtyRequest) (*ptypes.Empty, error) {
	internal.Log.Debugf("ResizePty: ID=%s, ExecID=%s", r.ID, r.ExecID)
	return s.runcService.ResizePty(ctx, r)
}

func (s *shiftpodService) CloseIO(ctx context.Context, r *taskAPI.CloseIORequest) (*ptypes.Empty, error) {
	internal.Log.Debugf("CloseIO: ID=%s, ExecID=%s", r.ID, r.ExecID)
	return s.runcService.CloseIO(ctx, r)
}

func (s *shiftpodService) Update(ctx context.Context, r *taskAPI.UpdateTaskRequest) (*ptypes.Empty, error) {
	internal.Log.Debugf("Update: ID=%s", r.ID)
	return s.runcService.Update(ctx, r)
}

func (s *shiftpodService) Wait(ctx context.Context, r *taskAPI.WaitRequest) (*taskAPI.WaitResponse, error) {
	internal.Log.Debugf("Wait: ID=%s, ExecID=%s", r.ID, r.ExecID)
	return s.runcService.Wait(ctx, r)
}

func (s *shiftpodService) Stats(ctx context.Context, r *taskAPI.StatsRequest) (*taskAPI.StatsResponse, error) {
	internal.Log.Debugf("Stats: ID=%s", r.ID)
	return s.runcService.Stats(ctx, r)
}

func (s *shiftpodService) Connect(ctx context.Context, r *taskAPI.ConnectRequest) (*taskAPI.ConnectResponse, error) {
	internal.Log.Debugf("Connect: ID=%s", r.ID)
	return s.runcService.Connect(ctx, r)
}

func (s *shiftpodService) Shutdown(ctx context.Context, r *taskAPI.ShutdownRequest) (*ptypes.Empty, error) {
	internal.Log.Debugf("Shutdown: ID=%s", r.ID)
	return s.runcService.Shutdown(ctx, r)
}
