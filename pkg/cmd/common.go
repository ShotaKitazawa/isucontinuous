package cmd

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	isucontinuousFilename = "isucontinuous.yaml"
)

type ConfigCommon struct {
	LogLevel      string
	LogFilename   string
	LocalRepoPath string
}

// TODO
func newLogger(logLevel, logfile string) (*zap.Logger, error) {
	zc := zap.NewProductionConfig()
	zc.Level = zap.NewAtomicLevelAt(zapcore.Level(-2))
	z, err := zc.Build()
	if err != nil {
		return nil, err
	}
	return z, nil
}
