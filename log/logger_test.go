package log

import (
	"testing"
)

func TestLogger(t *testing.T) {
	rootLogger.Error("test error level")
	rootLogger.Errorf("test error level %s", "format")
	rootLogger.Errorw("test error level", "ctx", "error")
	rootLogger.Info("test info level")
	rootLogger.Infof("test info level %s", "format")
	rootLogger.Infow("test info level", "ctx", "info")
	rootLogger.Warn("test info level")
	rootLogger.Warnf("test info level %s", "format")
	rootLogger.Warnw("test info level", "ctx", "info")
}
