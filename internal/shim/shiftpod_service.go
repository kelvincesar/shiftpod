package shim

import (
	"context"
	"fmt"
	"os"
	"sync"

	taskAPI "github.com/containerd/containerd/api/runtime/task/v3"
	ptypes "github.com/containerd/containerd/v2/pkg/protobuf/types"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/kelvinc/shiftpod/internal"
	pb "github.com/kelvinc/shiftpod/proto"
)

type shiftpodService struct {
	runcService        taskAPI.TTRPCTaskService
	mut                sync.Mutex
	shiftpodContainers map[string]*ShiftpodContainer
	managerClient      *managerClient
}

// Builder for the wrapper
func NewShiftpodService(runcService taskAPI.TTRPCTaskService) (taskAPI.TTRPCTaskService, error) {
	if runcService == nil {
		log.L.Error("Cannot initialize: underlying runc service is nil")
		return nil, fmt.Errorf("underlying runc service cannot be nil")
	}

	// Initialize manager client
	managerClient := NewManagerClient(DefaultManagerSocket)

	log.L.Info("Shiftpod wrapper initialized successfully")
	return &shiftpodService{
		runcService:        runcService,
		shiftpodContainers: make(map[string]*ShiftpodContainer),
		managerClient:      managerClient,
	}, nil
}

func (s *shiftpodService) setContainer(container *ShiftpodContainer) {
	s.mut.Lock()
	defer s.mut.Unlock()
	name := container.cfg.ContainerName
	s.shiftpodContainers[name] = container
}

func (s *shiftpodService) getContainerById(id string) (*ShiftpodContainer, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	if id == "" {
		return nil, fmt.Errorf("container ID cannot be empty")
	}
	if len(s.shiftpodContainers) == 0 {
		return nil, fmt.Errorf("no containers found")
	}

	for _, container := range s.shiftpodContainers {
		if container.ID == id {
			return container, nil
		}
	}
	return nil, fmt.Errorf("container with ID %s not found", id)
}
func (s *shiftpodService) getContainer(name string) (*ShiftpodContainer, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	if name == "" {
		return nil, fmt.Errorf("container name cannot be empty")
	}
	if len(s.shiftpodContainers) == 0 {
		return nil, fmt.Errorf("no containers found")
	}

	if container, ok := s.shiftpodContainers[name]; ok {
		internal.Log.Debugf("Found container with name %s: %+v", name, container)
		return container, nil
	}
	return nil, fmt.Errorf("container with name %s not found", name)
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

	var checkpointPath string
	var foundCheckpoint bool

	internal.Log.Debugf("Creating container with name %s. Checkpoint is %t and hash %s", cfg.ContainerName, cfg.CheckpointEnabled(), cfg.PodTemplateHash)
	// Always query manager for checkpoint availability (local or migration)
	if cfg.CheckpointEnabled() {
		internal.Log.Debugf("Querying manager for checkpoint for container %s (template hash: %s)", cfg.ContainerName, cfg.PodTemplateHash)

		if resp, err := s.managerClient.RequestMigrationRestore(ctx, cfg.PodTemplateHash, cfg.PodName, cfg.ContainerName); err == nil && resp.Found {
			checkpointPath = resp.CheckpointPath
			foundCheckpoint = true
			internal.Log.Infof("Found checkpoint from manager: %s", checkpointPath)
		} else if err != nil {
			internal.Log.Warnf("Failed to query manager for checkpoint: %v", err)
		} else {
			internal.Log.Debugf("No checkpoint available from manager for container %s", cfg.ContainerName)
		}
	}

	// Use checkpoint if found and checkpoint is enabled
	if foundCheckpoint && cfg.CheckpointEnabled() {
		internal.Log.Debugf("Container %s has checkpoint enabled, using restore path: %s", cfg.ContainerName, checkpointPath)
		restoreReq := &taskAPI.CreateTaskRequest{
			ID:         r.ID,
			Bundle:     r.Bundle,
			Rootfs:     r.Rootfs,
			Terminal:   r.Terminal,
			Stdin:      r.Stdin,
			Stdout:     r.Stdout,
			Stderr:     r.Stderr,
			Checkpoint: checkpointPath,
		}

		// Call runc service to create the container with checkpoint
		resp, err := s.runcService.Create(ctx, restoreReq)

		// Notify manager of restore completion
		restoreSuccess := err == nil
		if notifyErr := s.managerClient.FinishRestore(ctx, r.ID, restoreSuccess); notifyErr != nil {
			internal.Log.Warnf("Failed to notify manager of restore completion: %v", notifyErr)
		}

		if err != nil {
			if errdefs.IsNotImplemented(err) {
				internal.Log.Info("Restore not implemented by underlying shim")
			} else {
				internal.Log.Errorf("Restore failed: %v", err)
			}
		} else {
			internal.Log.Infof("Successfully restored container %s from checkpoint", r.ID)
		}

		return resp, err
	}

	// Create container without checkpoint
	container := NewShiftpodContainer(ctx, r.ID, cfg)
	s.setContainer(container)
	internal.Log.Debugf("Create: ID=%s, Bundle=%s, Config=%+v", r.ID, r.Bundle, cfg)

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
	container, err := s.getContainerById(r.ID)
	if err != nil {
		internal.Log.Errorf("Kill - failed to get container %s: %v", r.ID, err)
		return s.runcService.Kill(ctx, r)
	}

	// Check if it has checkpoint enabled
	if container.cfg.CheckpointEnabled() {
		path := container.createCheckpointPath()
		internal.Log.Debugf("Kill: ID=%s, ExecID=%s, Checkpoint path: %s", r.ID, r.ExecID, path)
		// https://github.com/containerd/containerd/blob/v2.1.1/cmd/containerd-shim-runc-v2/runc/container.go#L229

		_, err = s.runcService.Checkpoint(ctx, &taskAPI.CheckpointTaskRequest{
			ID:   r.ID,
			Path: path,
		})

		if err != nil {
			internal.Log.Errorf("Failed to checkpoint container %s: %v", r.ID, err)
			// move criu log to tmp
			moveCriuLog(r.ID)
		} else {
			internal.Log.Debugf("Checkpointed container %s successfully", r.ID)

			// Notify manager of checkpoint creation
			podInfo := &pb.PodInfo{
				Name:          container.cfg.PodName,
				Namespace:     container.cfg.PodNamespace,
				ContainerName: container.cfg.ContainerName,
				TemplateHash:  container.cfg.PodTemplateHash,
			}

			if notifyErr := s.managerClient.NotifyCheckpoint(ctx, r.ID, path, podInfo); notifyErr != nil {
				internal.Log.Warnf("Failed to notify manager of checkpoint creation: %v", notifyErr)
				// Continue anyway - fallback to local storage
			} else {
				internal.Log.Infof("Successfully notified manager of checkpoint for container %s", r.ID)
			}
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

	// Close manager client connection
	if s.managerClient != nil {
		if err := s.managerClient.Close(); err != nil {
			internal.Log.Warnf("Failed to close manager client: %v", err)
		}
	}

	return s.runcService.Shutdown(ctx, r)
}

func moveCriuLog(id string) error {
	src := fmt.Sprintf("/run/k3s/containerd/io.containerd.runtime.v2.task/k8s.io/%s/criu-dump.log", id)
	dst := fmt.Sprintf("/tmp/shiftpod/criu-dump-%s.log", id)
	internal.Log.Infof("Criu log moved to %s", dst)
	return os.Rename(src, dst)
}
