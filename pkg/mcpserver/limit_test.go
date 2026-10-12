package mcpserver

import (
	"strings"
	"testing"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

func artifact(name, content string) cmd.Artifact {
	return cmd.Artifact{Host: "web01", Path: "/p/" + name, Size: len(content), Content: content}
}

func lines(n int, width int) string {
	return strings.Repeat(strings.Repeat("x", width-1)+"\n", n)
}

func TestLimitArtifacts(t *testing.T) {
	limits := Limits{MaxFileBytes: 100, MaxResponseBytes: 250}

	t.Run("exactly the limit is kept whole", func(t *testing.T) {
		in := &cmd.AfterBenchResult{Artifacts: []cmd.Artifact{artifact("a", lines(10, 10))}}
		got := limitArtifacts(in, limits)
		if a := got.Artifacts[0]; a.Content != in.Artifacts[0].Content || a.Note != "" {
			t.Errorf("artifact = %+v", a)
		}
	})
	t.Run("one byte over keeps whole lines from the top", func(t *testing.T) {
		in := &cmd.AfterBenchResult{Artifacts: []cmd.Artifact{artifact("a", lines(10, 10)+"y")}}
		a := limitArtifacts(in, limits).Artifacts[0]
		if a.Content != lines(10, 10) {
			t.Errorf("content = %q", a.Content)
		}
		for _, want := range []string{"truncated", "kept 100 of 101 bytes", "web01:/p/a"} {
			if !strings.Contains(a.Note, want) {
				t.Errorf("note %q lacks %q", a.Note, want)
			}
		}
		if a.Size != 101 {
			t.Errorf("size = %d, want the original 101", a.Size)
		}
	})
	t.Run("never cuts inside a line", func(t *testing.T) {
		in := &cmd.AfterBenchResult{Artifacts: []cmd.Artifact{artifact("a", lines(3, 40))}}
		a := limitArtifacts(in, limits).Artifacts[0]
		if a.Content != lines(2, 40) || !strings.Contains(a.Note, "kept 80 of 120") {
			t.Errorf("content = %q, note = %q", a.Content, a.Note)
		}
	})
	t.Run("a single line over the limit leaves nothing and says so", func(t *testing.T) {
		in := &cmd.AfterBenchResult{Artifacts: []cmd.Artifact{artifact("a", strings.Repeat("x", 500)+"\n")}}
		a := limitArtifacts(in, limits).Artifacts[0]
		if a.Content != "" || !strings.Contains(a.Note, "kept 0 of 501") {
			t.Errorf("content = %q, note = %q", a.Content, a.Note)
		}
	})
	t.Run("a last line without a newline is cut as a line", func(t *testing.T) {
		in := &cmd.AfterBenchResult{Artifacts: []cmd.Artifact{artifact("a", lines(5, 10)+strings.Repeat("x", 80))}}
		a := limitArtifacts(in, limits).Artifacts[0]
		if a.Content != lines(5, 10) {
			t.Errorf("content = %q", a.Content)
		}
	})
	t.Run("the total shortens later files and omits the rest", func(t *testing.T) {
		in := &cmd.AfterBenchResult{Artifacts: []cmd.Artifact{
			artifact("a", lines(10, 10)), // 100
			artifact("b", lines(10, 10)), // 100
			artifact("c", lines(10, 10)), // 50 left
			artifact("d", lines(10, 10)), // 0 left
		}}
		got := limitArtifacts(in, limits).Artifacts
		if got[0].Note != "" || got[1].Note != "" {
			t.Errorf("notes = %q, %q", got[0].Note, got[1].Note)
		}
		if got[2].Content != lines(5, 10) || !strings.Contains(got[2].Note, "kept 50 of 100") {
			t.Errorf("c = %+v", got[2])
		}
		if got[3].Content != "" || !strings.HasPrefix(got[3].Note, "omitted") || !strings.Contains(got[3].Note, "web01:/p/d") {
			t.Errorf("d = %+v", got[3])
		}
	})
	t.Run("the total exactly reached keeps the last file", func(t *testing.T) {
		in := &cmd.AfterBenchResult{Artifacts: []cmd.Artifact{
			artifact("a", lines(10, 10)), artifact("b", lines(10, 10)), artifact("c", lines(5, 10)),
		}}
		got := limitArtifacts(in, limits).Artifacts
		if got[2].Note != "" || got[2].Content != lines(5, 10) {
			t.Errorf("c = %+v", got[2])
		}
	})
	t.Run("the input is not modified", func(t *testing.T) {
		in := &cmd.AfterBenchResult{Artifacts: []cmd.Artifact{artifact("a", lines(30, 10))}}
		limitArtifacts(in, limits)
		if in.Artifacts[0].Content != lines(30, 10) || in.Artifacts[0].Note != "" {
			t.Error("the full content must stay for the caller")
		}
	})
	t.Run("zero means the defaults", func(t *testing.T) {
		in := &cmd.AfterBenchResult{Artifacts: []cmd.Artifact{artifact("a", lines(DefaultMaxFileBytes/10+1, 10))}}
		a := limitArtifacts(in, Limits{}).Artifacts[0]
		if len(a.Content) > DefaultMaxFileBytes || a.Note == "" {
			t.Errorf("len = %d, note = %q", len(a.Content), a.Note)
		}
		if limitArtifacts(nil, Limits{}) != nil {
			t.Error("nil must stay nil")
		}
	})
}
