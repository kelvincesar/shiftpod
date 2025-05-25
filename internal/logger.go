package internal

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Entry

func SetupLogger() {
	baseLogger := logrus.New()

	f, _ := os.OpenFile("/tmp/shiftpod.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	baseLogger.SetOutput(f)

	baseLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	baseLogger.SetLevel(logrus.DebugLevel)

	// logger com campo fixo
	Log = baseLogger.WithField("", "sp")
}
