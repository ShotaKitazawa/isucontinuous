package slack

import (
	"context"

	"go.uber.org/zap"
)

type FakeClient struct {
	log *zap.Logger
}

func NewFakeClient(logger *zap.Logger) ClientIface {
	return &FakeClient{logger}
}

func (c FakeClient) SendText(ctx context.Context, channel, text string) error {
	return nil
}

func (c FakeClient) SendFileContent(ctx context.Context, channel, filename, content, title string) error {
	return nil
}
