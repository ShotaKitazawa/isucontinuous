package imports

import (
	"testing"

	"go.uber.org/zap"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
)

func TestNewImporters_KeyedByName(t *testing.T) {
	hosts := []config.Host{{Name: "isu1", Host: "192.0.2.1", Local: true}}
	importers, err := NewImporters(zap.NewNop(), hosts)
	if err != nil {
		t.Fatal(err)
	}
	if importers["isu1"] == nil || len(importers) != 1 {
		t.Errorf("importers = %v", importers)
	}
}
