// Copyright 2024 Authors of infrastructure-io
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelError LogLevel = "error"
)

type LogLevel string

func init() {
	InitStdoutLogger(LogLevelInfo)
}

// InitStdoutLogger initializes the global logger with the specified log level
func InitStdoutLogger(logLevel LogLevel) {
	if logLevel == "" {
		logLevel = LogLevelInfo // default log level
	}

	var level zapcore.Level
	switch logLevel {
	case LogLevelDebug:
		level = zapcore.DebugLevel
	case LogLevelInfo:
		level = zapcore.InfoLevel
	case LogLevelError:
		level = zapcore.ErrorLevel
	default:
		level = zapcore.DebugLevel
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}

	// Logger = logger.Sugar().Named("bmc")
	Logger = logger.Sugar()
	Logger.Infof("Logger initialized with level: %s", logLevel)
}
