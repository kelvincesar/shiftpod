package shim

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/kelvinc/shiftpod/internal"
	pb "github.com/kelvinc/shiftpod/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	DefaultManagerSocket = "/var/run/shiftpod/manager.sock"
	ConnectionTimeout    = 5 * time.Second
	MaxRetries           = 3
)

// ManagerServiceClient interface for gRPC communication
type ManagerServiceClient interface {
	NotifyCheckpoint(ctx context.Context, req *pb.NotifyCheckpointRequest) (*pb.NotifyCheckpointResponse, error)
	RequestMigrationRestore(ctx context.Context, req *pb.MigrationRestoreRequest) (*pb.MigrationRestoreResponse, error)
	FinishRestore(ctx context.Context, req *pb.FinishRestoreRequest) (*pb.FinishRestoreResponse, error)
	Close() error
}

// managerClient wraps the gRPC client with connection management
type managerClient struct {
	socketPath string
	conn       *grpc.ClientConn
	client     pb.ManagerServiceClient
	mu         sync.RWMutex
	connected  bool
}

// NewManagerClient creates a new manager client with Unix socket connection
func NewManagerClient(socketPath string) *managerClient {
	if socketPath == "" {
		socketPath = DefaultManagerSocket
	}

	return &managerClient{
		socketPath: socketPath,
	}
}

// connect establishes connection to the manager service
func (m *managerClient) connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connected && m.conn != nil {
		return nil
	}

	// Create connection to Unix socket
	conn, err := grpc.NewClient(
		"unix:///"+m.socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to manager at %s: %w", m.socketPath, err)
	}

	m.conn = conn
	m.client = pb.NewManagerServiceClient(conn)
	m.connected = true

	internal.Log.Infof("Connected to manager service at %s", m.socketPath)
	return nil
}

// ensureConnection ensures the client is connected, with retries
func (m *managerClient) ensureConnection(ctx context.Context) error {
	m.mu.RLock()
	if m.connected && m.conn != nil {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()

	var lastErr error
	for i := range MaxRetries {
		if err := m.connect(); err != nil {
			lastErr = err
			internal.Log.Debugf("Connection attempt %d failed: %v", i+1, err)
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		return nil
	}

	return fmt.Errorf("failed to connect after %d retries, last error: %w", MaxRetries, lastErr)
}

// NotifyCheckpoint notifies the manager when a checkpoint is created
func (m *managerClient) NotifyCheckpoint(ctx context.Context, containerID, checkpointPath string, podInfo *pb.PodInfo) error {
	if err := m.ensureConnection(ctx); err != nil {
		return fmt.Errorf("failed to connect to manager: %w", err)
	}

	req := &pb.NotifyCheckpointRequest{
		ContainerId:    containerID,
		CheckpointPath: checkpointPath,
		PodInfo:        podInfo,
	}

	m.mu.RLock()
	client := m.client
	m.mu.RUnlock()

	_, err := client.NotifyCheckpoint(ctx, req)
	if err != nil {
		m.handleConnectionError(err)
		return fmt.Errorf("failed to notify checkpoint: %w", err)
	}

	internal.Log.Infof("Successfully notified manager of checkpoint for container %s", containerID)
	return nil
}

// RequestMigrationRestore requests an available checkpoint from the manager
func (m *managerClient) RequestMigrationRestore(ctx context.Context, podTemplateHash, podName, containerName string) (*pb.MigrationRestoreResponse, error) {
	if err := m.ensureConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to manager: %w", err)
	}

	req := &pb.MigrationRestoreRequest{
		PodTemplateHash: podTemplateHash,
		PodName:         podName,
		ContainerName:   containerName,
	}

	m.mu.RLock()
	client := m.client
	m.mu.RUnlock()

	resp, err := client.RequestMigrationRestore(ctx, req)
	if err != nil {
		m.handleConnectionError(err)
		return nil, fmt.Errorf("failed to request migration restore: %w", err)
	}

	if resp.Found {
		internal.Log.Infof("Migration checkpoint found for pod %s, path: %s", podName, resp.CheckpointPath)
	} else {
		internal.Log.Debugf("No migration checkpoint found for pod template hash %s", podTemplateHash)
	}

	return resp, nil
}

// FinishRestore notifies the manager of restore completion status
func (m *managerClient) FinishRestore(ctx context.Context, containerID string, success bool) error {
	if err := m.ensureConnection(ctx); err != nil {
		// If we can't connect to report success/failure, log but don't fail
		internal.Log.Warnf("Failed to connect to manager to report restore status: %v", err)
		return nil
	}

	req := &pb.FinishRestoreRequest{
		ContainerId: containerID,
		Success:     success,
	}

	m.mu.RLock()
	client := m.client
	m.mu.RUnlock()

	_, err := client.FinishRestore(ctx, req)
	if err != nil {
		m.handleConnectionError(err)
		// Don't fail on restore notification errors
		internal.Log.Warnf("Failed to notify manager of restore completion: %v", err)
		return nil
	}

	internal.Log.Infof("Successfully notified manager of restore completion for container %s (success: %v)", containerID, success)
	return nil
}

// handleConnectionError handles connection errors by marking as disconnected
func (m *managerClient) handleConnectionError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connected {
		internal.Log.Warnf("Manager connection error, marking as disconnected: %v", err)
		m.connected = false
		if m.conn != nil {
			m.conn.Close()
			m.conn = nil
		}
	}
}

// Close closes the connection to the manager
func (m *managerClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn != nil {
		err := m.conn.Close()
		m.conn = nil
		m.client = nil
		m.connected = false
		return err
	}
	return nil
}

// IsConnected returns whether the client is currently connected
func (m *managerClient) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// Health checks if the manager is available
func (m *managerClient) Health(ctx context.Context) error {
	// Try to establish connection
	if err := m.ensureConnection(ctx); err != nil {
		return fmt.Errorf("manager health check failed: %w", err)
	}

	// Test with a simple call - we can use the finish restore with empty params
	// as a health check (it won't affect anything)
	req := &pb.FinishRestoreRequest{
		ContainerId: "health-check",
		Success:     true,
	}

	m.mu.RLock()
	client := m.client
	m.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := client.FinishRestore(ctx, req)
	if err != nil {
		m.handleConnectionError(err)
		return fmt.Errorf("manager health check failed: %w", err)
	}

	return nil
}

// Dialer creates a custom dialer for Unix socket connections
func unixDialer(ctx context.Context, addr string) (net.Conn, error) {
	return net.Dial("unix", addr)
}
