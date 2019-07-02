package log

import (
	"testing"

	"go.uber.org/zap"
)

func TestLogger(t *testing.T) {
	rootLogger.Error("test error level")
	rootLogger.Errorf("test error level %s", "format")
	rootLogger.Errorw("test error level", "ctx", "error")
	rootLogger.Info("test info level")
	rootLogger.Infof("test info level %s", "format")
	rootLogger.Infow("test info level", "ctx", "info", "hello", "world")
	rootLogger.Debug("test debug level (closed)")
	config.Level.SetLevel(zap.DebugLevel)
	rootLogger.Debug("test debug level (opened)")
	rootLogger.Warn("test info level")
	rootLogger.Warnf("test info level %s", "format")
	rootLogger.Warnw("test info level", "ctx", "info")
}
