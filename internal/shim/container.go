package shim

import (
	"context"
	"fmt"
	"os"
)

type ShiftpodContainer struct {
	ID             string
	cfg            *Config
	context        context.Context
	checkpointPath string
}

func NewShiftpodContainer(ctx context.Context, id string, cfg *Config) *ShiftpodContainer {
	return &ShiftpodContainer{
		ID:      id,
		cfg:     cfg,
		context: ctx,
	}
}

func (c *ShiftpodContainer) createCheckpointPath() string {
	if c.checkpointPath == "" {
		c.checkpointPath = fmt.Sprintf("/var/lib/shiftpod/checkpoints/%s", c.ID)
		os.MkdirAll(c.checkpointPath, 0755)
	}
	return c.checkpointPath
}
