package cmd

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/slack"
)

type newSlackClientFunc func(*config.Config) (slack.ClientIface, error)

func realSlackClient(logger *zap.Logger, token string) newSlackClientFunc {
	return func(c *config.Config) (slack.ClientIface, error) {
		return slack.NewClient(logger, token, c.Slack.DefaultChannelId)
	}
}

// slackNotifier returns a client that never fails: Slack is enabled by a non-empty default_channel_id,
// and when it is enabled but unusable the run continues with a warning, because a notification must not stop a deploy.
func slackNotifier(logger *zap.Logger, conf *config.Config, newClient newSlackClientFunc, warnings *warningList) slack.ClientIface {
	if conf.Slack.DefaultChannelId == "" {
		return slack.NewFakeClient(logger)
	}
	client, err := newClient(conf)
	if err != nil {
		msg := fmt.Sprintf("slack: notifications are skipped: %v", err)
		logger.Warn(msg)
		warnings.Add(msg)
		return slack.NewFakeClient(logger)
	}
	return &warnOnFailure{client, logger, warnings}
}

type warnOnFailure struct {
	slack.ClientIface
	logger   *zap.Logger
	warnings *warningList
}

func (w *warnOnFailure) SendText(ctx context.Context, channel, text string) error {
	if err := w.ClientIface.SendText(ctx, channel, text); err != nil {
		w.warn("failed to send a message", err)
	}
	return nil
}

func (w *warnOnFailure) SendFileContent(ctx context.Context, channel, filename, content, title string) error {
	if err := w.ClientIface.SendFileContent(ctx, channel, filename, content, title); err != nil {
		w.warn(fmt.Sprintf("failed to send %s", filename), err)
	}
	return nil
}

func (w *warnOnFailure) warn(what string, err error) {
	msg := fmt.Sprintf("slack: %s: %v", what, err)
	w.logger.Warn(msg)
	w.warnings.Add(msg)
}
