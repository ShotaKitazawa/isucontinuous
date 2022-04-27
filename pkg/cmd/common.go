package cmd

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"

	"github.com/ShotaKitazawa/isu-continuous/pkg/config"
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
	// setup encorder
	enc := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	// setup syncer
	f, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	sink := zapcore.AddSync(f)
	lsink := zapcore.Lock(sink)
	// setup log-level
	failedParseFlag := false
	level, err := zap.ParseAtomicLevel(logLevel)
	if err != nil || logLevel == "" {
		failedParseFlag = true
		level = zap.NewAtomicLevelAt(zapcore.Level(0)) // INFO
	}
	// new
	logger := zap.New(zapcore.NewCore(enc, lsink, level))
	if failedParseFlag {
		logger.Info("failed to parse log-level: set INFO")
	}
	return logger, nil
}

func perHostExec(logger *zap.Logger, ctx context.Context, hosts []config.Host, f func(context.Context, config.Host) error) error {
	eg, ctx := errgroup.WithContext(ctx)
	for _, host := range hosts {
		host := host
		eg.Go(func() error {
			// view.XXX
			if err := f(ctx, host); err != nil {
				logger.Error(err.Error(), zap.String("host", host.Host))
				return err
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}
