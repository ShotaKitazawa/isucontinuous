package cmd

import (
	"os"

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

func newLogger(logLevel, logfile string) (*zap.Logger, error) {
	f, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	level, err := zap.ParseAtomicLevel(logLevel)
	if err != nil {
		level = zap.NewAtomicLevelAt(zapcore.Level(0)) // INFO
	}
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(f),
		level.Level(),
	)
	return zap.New(core), nil
}
