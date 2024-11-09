package slack

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

type ClientIface interface {
	SendText(ctx context.Context, channel, text string) error
	SendFileContent(ctx context.Context, channel, filename, content, title string) error
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
		logger.Info(fmt.Sprintf("Slack client is not authorized. set fake client (nothing to notify): %v", err))
		return &FakeClient{logger}
	}
	return &Client{logger, api, channel}
}

func (c Client) SendText(ctx context.Context, channel, text string) error {
	if channel == "" {
		channel = c.defaultChannelId
	}
	if _, _, err := c.client.PostMessageContext(ctx, channel, slack.MsgOptionText(text, true)); err != nil {
		return err
	}
	return nil
}

func (c Client) SendFileContent(ctx context.Context, channel, filename, content, title string) error {
	if channel == "" {
		channel = c.defaultChannelId
	}
	params := slack.FileUploadParameters{
		Title:    title,
		Filetype: "txt",
		File:     filename,
		Content:  content,
		Channels: []string{channel},
	}
	file, err := c.client.UploadFile(params)
	if err != nil {
		return err
	}
	c.log.Debug(fmt.Sprintf("sent file %s to Slack", file.Name))
	return nil
}
