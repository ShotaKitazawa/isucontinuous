package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/cheggaaa/pb"
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

type taskFunc func(context.Context, config.Host) error

type task struct {
	name string
	f    taskFunc
}

func perHostExec(logger *zap.Logger, ctx context.Context, hosts []config.Host, tasks []task) error {
	eg, ctx := errgroup.WithContext(ctx)
	pbs := make([]*pb.ProgressBar, len(hosts))
	var mu sync.RWMutex
	for idx, host := range hosts {
		idx := idx
		host := host
		pbs[idx] = pb.New(len(tasks)).SetMaxWidth(80)
		eg.Go(func() error {
			for _, task := range tasks {
				mu.Lock()
				pbs[idx] = pbs[idx].Prefix(fmt.Sprintf("[%s] %s", host.Host, task.name))
				mu.Unlock()
				if err := task.f(ctx, host); err != nil {
					logger.Error(err.Error(), zap.String("host", host.Host))
					return err
				}
				mu.Lock()
				pbs[idx].Increment()
				mu.Unlock()
			}
			mu.Lock()
			pbs[idx].Prefix(fmt.Sprintf("[%s] %s", host.Host, "Done!"))
			mu.Unlock()
			return nil
		})
	}
	mu.RLock()
	pool, err := pb.StartPool(pbs...)
	mu.RUnlock()
	if err != nil {
		return err
	}
	defer func() {
		_ = pool.Stop()
	}()
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}
