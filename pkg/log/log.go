// Copyright 2024 Authors of infrastructure-io
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

// InitStdoutLogger initializes the global logger with the specified log level
func InitStdoutLogger(logLevel string) {
	if logLevel == "" {
		logLevel = "debug" // default log level
	}

	var level zapcore.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.DebugLevel
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      true,
		Encoding:        "console",
		EncoderConfig:   zap.NewDevelopmentEncoderConfig(),
		OutputPaths:     []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}

	//Logger = logger.Sugar().Named("bmc")
	Logger=logger.Sugar()
	Logger.Infof("Logger initialized with level: %s", logLevel)
}
