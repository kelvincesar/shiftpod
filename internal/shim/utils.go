package shim

import (
	"context"

	"github.com/containerd/log"
	"github.com/sirupsen/logrus"
)

func logger(ctx context.Context) *logrus.Entry {
	l := log.G(ctx)
	if l.Logger == nil {
		return log.L.WithField("fallback", true)
	}
	return l.WithField("component", "shiftpod")
}
