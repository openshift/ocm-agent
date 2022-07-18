package logging

import (
	"os"

	"github.com/sirupsen/logrus"
)

var (
	DebugLogLevel = logrus.DebugLevel
)

// NewLogger initializes logging with Info level logging as default
func NewLogger() (logger *logrus.Logger) {
	logger = &logrus.Logger{
		Out:       os.Stderr,
		Formatter: new(logrus.TextFormatter),
		Level:     logrus.InfoLevel,
	}
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		PadLevelText:  false,
	})
	return logger
}
