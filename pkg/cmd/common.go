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
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	//enc := zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig())

	f, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	sink := zapcore.AddSync(f)
	lsink := zapcore.Lock(sink)

	failedParseFlag := false
	level, err := zap.ParseAtomicLevel(logLevel)
	if err != nil {
		failedParseFlag = true
		level = zap.NewAtomicLevelAt(zapcore.Level(0)) // INFO
	}

	logger := zap.New(zapcore.NewCore(enc, lsink, level))
	if failedParseFlag {
		logger.Info("failed to parse log-level: set INFO")
	}
	return logger, nil
}
