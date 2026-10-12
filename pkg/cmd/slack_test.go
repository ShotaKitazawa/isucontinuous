package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/slack"
)

func TestSlackNotifier_DisabledWithoutChannel(t *testing.T) {
	called := false
	newClient := func(*config.Config) (slack.ClientIface, error) {
		called = true
		return nil, errors.New("must not be called")
	}
	warnings := &warningList{}
	client := slackNotifier(zaptest.NewLogger(t), &config.Config{}, newClient, warnings)
	if err := client.SendText(context.Background(), "", "hello"); err != nil {
		t.Fatal(err)
	}
	if called || len(warnings.List()) != 0 {
		t.Errorf("called = %v, warnings = %v, want neither", called, warnings.List())
	}
}

func TestSlackNotifier_UnusableClientWarns(t *testing.T) {
	conf := &config.Config{Slack: config.Slack{DefaultChannelId: "C123"}}
	newClient := func(*config.Config) (slack.ClientIface, error) { return nil, errors.New("slack token is not set") }
	warnings := &warningList{}
	client := slackNotifier(zaptest.NewLogger(t), conf, newClient, warnings)
	if err := client.SendText(context.Background(), "", "hello"); err != nil {
		t.Fatal(err)
	}
	got := warnings.List()
	if len(got) != 1 || !strings.Contains(got[0], "slack token is not set") {
		t.Errorf("warnings = %v", got)
	}
}

func TestSlackNotifier_SendFailureWarns(t *testing.T) {
	conf := &config.Config{Slack: config.Slack{DefaultChannelId: "C123"}}
	newClient := func(*config.Config) (slack.ClientIface, error) {
		return slack.NewFailingClient(errors.New("channel_not_found")), nil
	}
	warnings := &warningList{}
	client := slackNotifier(zaptest.NewLogger(t), conf, newClient, warnings)
	if err := client.SendText(context.Background(), "", "hello"); err != nil {
		t.Errorf("SendText = %v, want nil", err)
	}
	if err := client.SendFileContent(context.Background(), "", "alp.txt", "body", "title"); err != nil {
		t.Errorf("SendFileContent = %v, want nil", err)
	}
	got := warnings.List()
	if len(got) != 2 || !strings.Contains(got[0], "channel_not_found") || !strings.Contains(got[1], "alp.txt") {
		t.Errorf("warnings = %v", got)
	}
}
