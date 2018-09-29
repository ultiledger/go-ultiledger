package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var rootLogger *zap.SugaredLogger

func init() {
	config := zap.NewProductionConfig()
	// change stacktrace output level to DPanic so that
	// we will not get cluttered log in Error level
	stacktraceOption := zap.AddStacktrace(zapcore.DPanicLevel)
	callerOption := zap.AddCallerSkip(1)
	logger, err := config.Build(stacktraceOption, callerOption)
	if err != nil {
		panic(err)
	}
	rootLogger = logger.Sugar()
}

// Wrap the methods of global sugared logger for purpose
// of removing the ugly S() method call to write log
func Error(args ...interface{}) {
	rootLogger.Error(args)
}

func Errorf(template string, args ...interface{}) {
	rootLogger.Errorf(template, args)
}

func Errorw(msg string, keysAndValues ...interface{}) {
	rootLogger.Errorw(msg, keysAndValues)
}

func Fatal(args ...interface{}) {
	rootLogger.Fatal(args)
}

func Fatalf(template string, args ...interface{}) {
	rootLogger.Fatalf(template, args)
}

func Fatalw(msg string, keysAndValues ...interface{}) {
	rootLogger.Fatalw(msg, keysAndValues)
}

func Warn(args ...interface{}) {
	rootLogger.Warn(args)
}

func Warnf(template string, args ...interface{}) {
	rootLogger.Warnf(template, args)
}

func Warnw(msg string, keysAndValues ...interface{}) {
	rootLogger.Warnw(msg, keysAndValues)
}

func Info(args ...interface{}) {
	rootLogger.Info(args)
}

func Infof(template string, args ...interface{}) {
	rootLogger.Infof(template, args)
}

func Infow(msg string, keysAndValues ...interface{}) {
	rootLogger.Fatalw(msg, keysAndValues)
}
