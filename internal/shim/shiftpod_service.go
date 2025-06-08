package shim

import (
	"context"
	"fmt"
	"sync"

	taskAPI "github.com/containerd/containerd/api/runtime/task/v3"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/kelvinc/shiftpod/internal"
)

type shiftpodService struct {
	runcService        taskAPI.TTRPCTaskService
	mut                sync.Mutex
	shiftpodContainers map[string]*ShiftpodContainer
}

// Builder for the wrapper
func NewShiftpodService(runcService taskAPI.TTRPCTaskService) (taskAPI.TTRPCTaskService, error) {
	if runcService == nil {
		log.L.Error("Cannot initialize: underlying runc service is nil")
		return nil, fmt.Errorf("underlying runc service cannot be nil")
	}
	log.L.Info("Shiftpod wrapper initialized successfully")
	return &shiftpodService{
		runcService:        runcService,
		shiftpodContainers: make(map[string]*ShiftpodContainer),
	}, nil
}

func (s *shiftpodService) setContainer(id string, container *ShiftpodContainer) {
	s.mut.Lock()
	defer s.mut.Unlock()

	s.shiftpodContainers[id] = container
}

func (s *shiftpodService) getContainer(id string) (*ShiftpodContainer, error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	container, ok := s.shiftpodContainers[id]
	if !ok {
		return nil, fmt.Errorf("container %s not found", id)
	}
	return container, nil
}

func (s *shiftpodService) Create(ctx context.Context, r *taskAPI.CreateTaskRequest) (*taskAPI.CreateTaskResponse, error) {
	internal.Log.Infof("Create called: ID=%s, Bundle=%s", r.ID, r.Bundle)

	// Parse config and container spec
	spec, err := GetSpec(r.Bundle)
	if err != nil {
		return nil, err
	}
	cfg, err := NewConfig(ctx, spec)
	if err != nil {
		internal.Log.Errorf("Failed to create config: %v", err)
		return nil, err
	}

	// Store container information internaly
	container := NewShiftpodContainer(ctx, r.ID, cfg)
	s.setContainer(r.ID, container)
	internal.Log.Debugf("Create: ID=%s, Bundle=%s, Config=%+v", r.ID, r.Bundle, cfg)

	if cfg.CheckpointEnabled() {
		internal.Log.Debugf("Checkpoint enabled for container %s", r.ID)
	}

	// Call runc service to create the container
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
	// Get the container from the map
	container, err := s.getContainer(r.ID)
	if err != nil {
		internal.Log.Errorf("Failed to get container %s: %v", r.ID, err)
		return s.runcService.Kill(ctx, r)
	}

	// Check if it has checkpoint enabled
	if container.cfg.CheckpointEnabled() {
		path := container.createCheckpointPath()
		internal.Log.Debugf("Kill: ID=%s, ExecID=%s, Checkpoint path: %s", r.ID, r.ExecID, path)
		// https://github.com/containerd/containerd/blob/v2.1.1/cmd/containerd-shim-runc-v2/runc/container.go#L229
		/*options := &runctypes.CheckpointOptions{
			Exit:                true,
			OpenTcp:             true,
			ExternalUnixSockets: true,
			Terminal:            true,
			FileLocks:           true,
			CgroupsMode:         "soft",
			ImagePath:           path,
			WorkPath:            path,
		}

		raw, err := proto.Marshal(options)
		if err != nil {
			return nil, fmt.Errorf("falha ao serializar CheckpointOptions: %w", err)
		}
		anyOpts := &anypb.Any{
			TypeUrl: "type.googleapis.com/containerd.runc.v1.CheckpointOptions",
			Value:   raw,
		*/
		_, err = s.runcService.Checkpoint(ctx, &taskAPI.CheckpointTaskRequest{
			ID:   r.ID,
			Path: path,
		})

		if err != nil {

			internal.Log.Errorf("Failed to checkpoint container %s: %v", r.ID, err)
		} else {
			internal.Log.Debugf("Checkpointed container %s successfully", r.ID)
		}
	} else {
		internal.Log.Debugf("Kill: ID=%s, ExecID=%s, Checkpoint not enabled", r.ID, r.ExecID)
	}

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
