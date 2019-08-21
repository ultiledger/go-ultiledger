// Copyright 2019 The go-ultiledger Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
