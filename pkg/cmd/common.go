package cmd

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

type ConfigCommon struct {
	LogLevel       string
	LogFilename    string
	LocalRepoPath  string
	KnownHostsPath string
	Reporter       Reporter
}

func (c ConfigCommon) reporter() Reporter {
	if c.Reporter == nil {
		return NewProgressReporter()
	}
	return c.Reporter
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

func perHostExec(
	logger *zap.Logger, ctx context.Context, reporter Reporter, hosts []config.Host, tasks []task,
) ([]HostResult, error) {
	names := make([]string, len(hosts))
	results := make([]HostResult, len(hosts))
	for idx, host := range hosts {
		names[idx] = host.Name
		results[idx] = HostResult{Host: host.Name}
	}
	if err := reporter.Open(names, len(tasks)); err != nil {
		return results, err
	}
	defer reporter.Close()
	eg, ctx := errgroup.WithContext(ctx)
	for idx, host := range hosts {
		idx := idx
		host := host
		eg.Go(func() error {
			for _, task := range tasks {
				reporter.TaskStarted(host.Name, task.name)
				if err := task.f(ctx, host); err != nil {
					logger.Error(err.Error(), zap.String("host", host.Name))
					reporter.TaskFailed(host.Name, task.name, err)
					return myerrors.NewErrorHostTask(host.Name, task.name, err)
				}
				reporter.TaskDone(host.Name, task.name)
				results[idx].Tasks = append(results[idx].Tasks, task.name)
			}
			return nil
		})
	}
	err := eg.Wait()
	return results, err
}
