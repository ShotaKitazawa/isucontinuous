package slack

import (
	"context"

	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

type Client struct {
	log              *zap.Logger
	client           *slack.Client
	defaultChannelId string
}

func NewClient(logger *zap.Logger, token, channel string) *Client {
	api := slack.New(token)
	// TODO: verify token
	// TODO: get channel id
	return &Client{logger, api, channel}
}

func (c Client) SendText(ctx context.Context, channel, text string) error {
	channelId := c.defaultChannelId
	if channel != "" {
		// TODO: get channel id
	}
	if _, _, err := c.client.PostMessageContext(ctx, channelId, slack.MsgOptionText(text, false)); err != nil {
		return err
	}
	return nil
}
