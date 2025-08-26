package shim

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kelvinc/shiftpod/internal"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	MigrateAnnotation          = "shiftpod/migrate"
	EnableCheckpointAnnotation = "shiftpod/enable-checkpoint"
	CRIContainerNameAnnotation = "io.kubernetes.cri.container-name"
	CRIContainerTypeAnnotation = "io.kubernetes.cri.container-type"
	PodNameAnnotation          = "io.kubernetes.pod.name"
	PodNamespaceAnnotation     = "io.kubernetes.pod.namespace"
	PodTemplateHashLabel       = "pod-template-hash"
)

type Config struct {
	spec             *specs.Spec
	EnableCheckpoint bool
	EnableMigrate    bool
	ContainerName    string
	containerType    string
	PodName          string
	PodNamespace     string
	PodTemplateHash  string
}

// Parse specs received from containerd
func NewConfig(ctx context.Context, spec *specs.Spec) (*Config, error) {
	containerName := spec.Annotations[CRIContainerNameAnnotation]
	containerType := spec.Annotations[CRIContainerTypeAnnotation]
	podName := spec.Annotations[PodNameAnnotation]
	podNamespace := spec.Annotations[PodNamespaceAnnotation]

	// Debug: Log all available annotations
	if spec.Annotations != nil {
		internal.Log.Debugf("Available annotations:")
		for key, value := range spec.Annotations {
			internal.Log.Debugf("  %s = %s", key, value)
		}
	}

	// Extract pod template hash from annotations if available
	var podTemplateHash string
	if spec.Annotations != nil {
		// Look for pod template hash in various annotation formats
		if val, ok := spec.Annotations["io.kubernetes.pod.template-hash"]; ok {
			podTemplateHash = val
		} else if val, ok := spec.Annotations["pod-template-hash"]; ok {
			podTemplateHash = val
		} else if val, ok := spec.Annotations["io.kubernetes.pod.uid"]; ok {
			// Fallback: try to extract from pod UID or name
			internal.Log.Debugf("Pod UID: %s", val)
		}
		// Check for Kubernetes labels passed as annotations
		for key, value := range spec.Annotations {
			if strings.Contains(key, "template-hash") || strings.Contains(key, "pod-template") {
				internal.Log.Debugf("Found template hash candidate: %s = %s", key, value)
				if podTemplateHash == "" {
					podTemplateHash = value
				}
			}
		}
	}

	config := Config{
		spec:             spec,
		ContainerName:    containerName,
		containerType:    containerType,
		PodName:          podName,
		PodNamespace:     podNamespace,
		PodTemplateHash:  podTemplateHash,
		EnableCheckpoint: false,
		EnableMigrate:    false,
	}

	if spec.Annotations == nil {
		return &config, nil
	}

	if val, ok := spec.Annotations[EnableCheckpointAnnotation]; ok {
		internal.Log.Debugf("EnableCheckpointAnnotation: %s", val)
		if val == "true" {
			config.EnableCheckpoint = true

		}
	}

	if val, ok := spec.Annotations[MigrateAnnotation]; ok && val == "true" {
		config.EnableMigrate = true
	}

	internal.Log.Debugf("Config: EnableCheckpoint=%v, EnableMigrate=%v, containerName=%s, containerType=%s, podName=%s, podNamespace=%s, podTemplateHash=%s",
		config.EnableCheckpoint, config.EnableMigrate, config.ContainerName, config.containerType, config.PodName, config.PodNamespace, config.PodTemplateHash)
	return &config, nil
}

func (c *Config) CheckpointEnabled() bool {
	return c.EnableCheckpoint
}

func GetSpec(bundlePath string) (*specs.Spec, error) {
	var bundleSpec specs.Spec
	bundleConfigContents, err := os.ReadFile(filepath.Join(bundlePath, "config.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read budle: %w", err)
	}

	if err := json.Unmarshal(bundleConfigContents, &bundleSpec); err != nil {
		return nil, err
	}

	return &bundleSpec, nil
}
