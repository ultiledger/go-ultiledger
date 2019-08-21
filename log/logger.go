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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var rootLogger *zap.SugaredLogger
var config zap.Config

func init() {
	config = zap.NewProductionConfig()
	// Change stacktrace output level to DPanic for having
	// a cleaner error message in Error level.
	stacktraceOption := zap.AddStacktrace(zapcore.DPanicLevel)
	callerOption := zap.AddCallerSkip(1)
	logger, err := config.Build(stacktraceOption, callerOption)
	if err != nil {
		panic(err)
	}
	rootLogger = logger.Sugar()
}

func OpenDebug() {
	config.Level.SetLevel(zap.DebugLevel)
}

func CloseDebug() {
	config.Level.SetLevel(zap.InfoLevel)
}

// Wrap the methods of global sugared logger for purpose
// of removing the ugly S() method call to write log
func Error(args ...interface{}) {
	rootLogger.Error(args...)
}

func Errorf(template string, args ...interface{}) {
	rootLogger.Errorf(template, args...)
}

func Errorw(msg string, keysAndValues ...interface{}) {
	rootLogger.Errorw(msg, keysAndValues...)
}

func Fatal(args ...interface{}) {
	rootLogger.Fatal(args...)
}

func Fatalf(template string, args ...interface{}) {
	rootLogger.Fatalf(template, args...)
}

func Fatalw(msg string, keysAndValues ...interface{}) {
	rootLogger.Fatalw(msg, keysAndValues...)
}

func Warn(args ...interface{}) {
	rootLogger.Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	rootLogger.Warnf(template, args...)
}

func Warnw(msg string, keysAndValues ...interface{}) {
	rootLogger.Warnw(msg, keysAndValues...)
}

func Info(args ...interface{}) {
	rootLogger.Info(args...)
}

func Infof(template string, args ...interface{}) {
	rootLogger.Infof(template, args...)
}

func Infow(msg string, keysAndValues ...interface{}) {
	rootLogger.Infow(msg, keysAndValues...)
}

func Debug(args ...interface{}) {
	rootLogger.Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	rootLogger.Debugf(template, args...)
}

func Debugw(msg string, keysAndValues ...interface{}) {
	rootLogger.Debugw(msg, keysAndValues...)
}
