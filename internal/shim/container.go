package shim

import "context"

type ShiftpodContainer struct {
	ID      string
	cfg     *Config
	context context.Context
}

func NewShiftpodContainer(ctx context.Context, id string, cfg *Config) *ShiftpodContainer {
	return &ShiftpodContainer{
		ID:      id,
		cfg:     cfg,
		context: ctx,
	}
}
