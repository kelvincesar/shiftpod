package shim

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kelvinc/shiftpod/internal"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	MigrateAnnotation          = "shiftpod/migrate"
	EnableCheckpointAnnotation = "shiftpod/enable-checkpoint"
	CRIContainerNameAnnotation = "io.kubernetes.cri.container-name"
	CRIContainerTypeAnnotation = "io.kubernetes.cri.container-type"
)

type Config struct {
	spec             *specs.Spec
	EnableCheckpoint bool
	EnableMigrate    bool
	ContainerName    string
	containerType    string
}

// Parse specs received from containerd
func NewConfig(ctx context.Context, spec *specs.Spec) (*Config, error) {
	containerName := spec.Annotations[CRIContainerNameAnnotation]
	containerType := spec.Annotations[CRIContainerTypeAnnotation]
	config := Config{
		spec:             spec,
		ContainerName:    containerName,
		containerType:    containerType,
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

	internal.Log.Debugf("Config: EnableCheckpoint=%v, EnableMigrate=%v, containerName=%s, containerType=%s",
		config.EnableCheckpoint, config.EnableMigrate, config.ContainerName, config.containerType)
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
