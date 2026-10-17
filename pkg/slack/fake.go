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

// FailingClient fails every send with err.
type FailingClient struct {
	err error
}

func NewFailingClient(err error) ClientIface {
	return &FailingClient{err}
}

func (c FailingClient) SendText(ctx context.Context, channel, text string) error {
	return c.err
}

func (c FailingClient) SendFileContent(ctx context.Context, channel, filename, content, title string) error {
	return c.err
}
