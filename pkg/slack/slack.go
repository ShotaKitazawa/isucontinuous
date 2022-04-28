package slack

import (
	"context"

	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

type ClientIface interface {
	SendText(ctx context.Context, channel, text string) error
}

type Client struct {
	log              *zap.Logger
	client           *slack.Client
	defaultChannelId string
}

func NewClient(logger *zap.Logger, token, channel string) ClientIface {
	api := slack.New(token)
	if _, err := api.AuthTest(); err != nil {
		// set fake client
		logger.Info("Slack client is not authorized. set fake client (nothing to notify)")
		return &FakeClient{logger}
	}
	return &Client{logger, api, channel}
}

func (c Client) SendText(ctx context.Context, channel, text string) error {
	channelId := c.defaultChannelId
	if channel == "" {
		channel = c.defaultChannelId
	}
	if _, _, err := c.client.PostMessageContext(ctx, channelId, slack.MsgOptionText(text, true)); err != nil {
		return err
	}
	return nil
}
