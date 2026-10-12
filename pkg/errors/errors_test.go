package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestHostTask(t *testing.T) {
	inner := NewErrorGitBranchIsDetached()
	err := fmt.Errorf("wrapped: %w", NewErrorHostTask("host01", "Deploy", inner))

	var ht HostTask
	if !errors.As(err, &ht) || ht.Host != "host01" || ht.Task != "Deploy" {
		t.Errorf("HostTask not extracted: %+v", ht)
	}
	if !errors.As(err, &GitBranchIsDetached{}) {
		t.Errorf("GitBranchIsDetached is not extracted through HostTask")
	}
	if got, want := NewErrorHostTask("h", "t", inner).Error(), inner.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
