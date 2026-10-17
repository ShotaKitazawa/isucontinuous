package install

import (
	"testing"

	"go.uber.org/zap"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
)

func TestNewInstallers_KeyedByName(t *testing.T) {
	hosts := []config.Host{{Name: "isu1", Host: "192.0.2.1", Local: true}}
	installers, err := NewInstallers(zap.NewNop(), hosts)
	if err != nil {
		t.Fatal(err)
	}
	if installers["isu1"] == nil || len(installers) != 1 {
		t.Errorf("installers = %v", installers)
	}
}
