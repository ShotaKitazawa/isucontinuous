package cmd

import (
	"fmt"
	"os"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
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

func perHostExec(logger *zap.Logger, hosts []config.Host, f func(config.Host) error) error {
	var eg errgroup.Group
	for _, host := range hosts {
		host := host
		eg.Go(func() error {
			// view.XXX
			if err := f(host); err != nil {
				logger.Error(fmt.Sprintf("in host %s:\n%v", host.Host, err))
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil { // 実行が終わるまで待つ
		return err
	}
	return nil
}
