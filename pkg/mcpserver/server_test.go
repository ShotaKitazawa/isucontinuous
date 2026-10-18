package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

func connect(t *testing.T, opts Options, handler func(context.Context, *mcp.ProgressNotificationClientRequest)) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := New(opts).Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test"}, &mcp.ClientOptions{ProgressNotificationHandler: handler})
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func listTools(t *testing.T, cs *mcp.ClientSession) map[string]*mcp.Tool {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	tools := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		tools[tool.Name] = tool
	}
	return tools
}

func properties(t *testing.T, tool *mcp.Tool) map[string]any {
	t.Helper()
	b, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatal(err)
	}
	return schema.Properties
}

func TestListToolsMatchesCLI(t *testing.T) {
	tools := listTools(t, connect(t, Options{}, nil))
	var names []string
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	want := []string{"afterbench", "deploy", "diff", "import", "init", "profiling", "push", "release", "setup", "status", "sync"}
	if len(names) != len(want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("tools = %v, want %v", names, want)
		}
	}
}

func TestForceAndDriftExposure(t *testing.T) {
	tools := listTools(t, connect(t, Options{}, nil))
	has := func(tool, prop string) bool { _, ok := properties(t, tools[tool])[prop]; return ok }
	for _, c := range []struct {
		tool, prop string
		want       bool
	}{
		{"sync", "force", false},
		{"deploy", "force", true},
		{"deploy", "ignore_drift", true},
		{"release", "force", true},
		{"setup", "force", false},
		{"import", "force", false},
		{"push", "message", true},
	} {
		if got := has(c.tool, c.prop); got != c.want {
			t.Errorf("%s.%s exposed = %v, want %v", c.tool, c.prop, got, c.want)
		}
	}
}

func TestDescriptionsCarryRules(t *testing.T) {
	tools := listTools(t, connect(t, Options{}, nil))
	for tool, phrases := range map[string][]string{
		"deploy":  {"ask the holder first", "ignore_drift", "read them and act"},
		"release": {"ask the holder first"},
	} {
		for _, p := range phrases {
			if !contains(tools[tool].Description, p) {
				t.Errorf("%s description lacks %q", tool, p)
			}
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestCallReturnsStructuredResultWithWarnings(t *testing.T) {
	var got cmd.ConfigDeploy
	runners := DefaultRunners()
	runners.Deploy = func(_ context.Context, c cmd.ConfigDeploy) (*cmd.DeployResult, error) {
		got = c
		return &cmd.DeployResult{
			Result: cmd.Result{
				Hosts:    []cmd.HostResult{{Host: "host1", Tasks: []string{"deploy"}}},
				Warnings: []string{"slack is disabled"},
			},
			Hash: "abc123",
		}, nil
	}
	cs := connect(t, Options{Runners: &runners, SlackToken: "tok"}, nil)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "deploy",
		Arguments: map[string]any{"revision": "v1", "ignore_drift": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %+v", res.Content)
	}
	if got.GitRevision != "v1" || !got.IgnoreDrift || got.Force || got.SlackToken != "tok" {
		t.Errorf("config = %+v", got)
	}
	if got.Owner != "" {
		t.Errorf("owner = %q, want the CLI default (empty)", got.Owner)
	}
	b, _ := json.Marshal(res.StructuredContent)
	var out cmd.DeployResult
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Hash != "abc123" || len(out.Warnings) != 1 || out.Warnings[0] != "slack is disabled" || out.Hosts[0].Host != "host1" {
		t.Errorf("result = %+v", out)
	}
}

func TestCallDefaultsBranchToMaster(t *testing.T) {
	var got cmd.ConfigSync
	runners := DefaultRunners()
	runners.Sync = func(_ context.Context, c cmd.ConfigSync) (*cmd.SyncResult, error) {
		got = c
		return &cmd.SyncResult{}, nil
	}
	cs := connect(t, Options{Runners: &runners}, nil)
	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "sync", Arguments: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if got.GitBranch != "master" || got.Force {
		t.Errorf("config = %+v", got)
	}
}

func TestCallPropagatesCancellationToRunner(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	runners := DefaultRunners()
	runners.Sync = func(ctx context.Context, _ cmd.ConfigSync) (*cmd.SyncResult, error) {
		close(started)
		<-ctx.Done()
		close(canceled)
		return nil, ctx.Err()
	}
	cs := connect(t, Options{Runners: &runners}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = cs.CallTool(ctx, &mcp.CallToolParams{Name: "sync", Arguments: map[string]any{}})
	}()
	<-started
	cancel()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("runner context was not canceled")
	}
	<-done
}

func TestCallErrorCarriesKindHostTaskAndPartialResult(t *testing.T) {
	runners := DefaultRunners()
	runners.Deploy = func(_ context.Context, c cmd.ConfigDeploy) (*cmd.DeployResult, error) {
		return &cmd.DeployResult{Result: cmd.Result{Hosts: []cmd.HostResult{{Host: "host1", Tasks: []string{"lock"}}}}},
			myerrors.NewErrorHostTask("host2", "deploy", errors.New("boom"))
	}
	cs := connect(t, Options{Runners: &runners}, nil)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "deploy", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("want an error result")
	}
	var te toolError
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &te); err != nil {
		t.Fatal(err)
	}
	if te.Host != "host2" || te.Task != "deploy" || te.Message != "boom" || te.Kind == "" {
		t.Errorf("error = %+v", te)
	}
	if res.StructuredContent == nil {
		t.Error("partial result was dropped")
	}
}

func TestProgressNotifiedOnlyWithToken(t *testing.T) {
	runners := DefaultRunners()
	runners.Status = func(_ context.Context, c cmd.ConfigStatus) (*cmd.StatusResult, error) {
		r := c.Reporter
		_ = r.Open([]string{"host1"}, 1)
		r.TaskStarted("host1", "status")
		r.TaskDone("host1", "status")
		r.Close()
		return &cmd.StatusResult{}, nil
	}
	var mu sync.Mutex
	var msgs []*mcp.ProgressNotificationParams
	cs := connect(t, Options{Runners: &runners}, func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
		mu.Lock()
		defer mu.Unlock()
		msgs = append(msgs, req.Params)
	})
	ctx := context.Background()
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "status", Arguments: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(msgs) != 0 {
		t.Fatalf("notified without a token: %v", msgs)
	}
	mu.Unlock()

	// SetProgressToken of v1.3.1 drops the token when Meta is nil
	params := &mcp.CallToolParams{Name: "status", Arguments: map[string]any{}, Meta: mcp.Meta{"progressToken": "tok"}}
	if _, err := cs.CallTool(ctx, params); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		mu.Lock()
		n := len(msgs)
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(msgs) != 2 {
		t.Fatalf("notifications = %d, want 2", len(msgs))
	}
	if msgs[0].Progress >= msgs[1].Progress || msgs[1].Progress != msgs[1].Total || msgs[0].ProgressToken != "tok" {
		t.Errorf("notifications = %+v %+v", msgs[0], msgs[1])
	}
}

func TestAfterBenchResponseIsLimited(t *testing.T) {
	full := strings.Repeat("0123456789\n", 10)
	runners := DefaultRunners()
	runners.AfterBench = func(context.Context, cmd.ConfigAfterBench) (*cmd.AfterBenchResult, error) {
		return &cmd.AfterBenchResult{Artifacts: []cmd.Artifact{
			{Host: "web01", Path: "/p/abc.1/alp.txt", Size: len(full), Content: full},
		}}, nil
	}
	cs := connect(t, Options{Runners: &runners, Limits: Limits{MaxFileBytes: 33}}, nil)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "afterbench"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res.StructuredContent)
	var out cmd.AfterBenchResult
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	a := out.Artifacts[0]
	if a.Content != strings.Repeat("0123456789\n", 3) || a.Size != len(full) ||
		!strings.Contains(a.Note, "kept 33 of 110 bytes") || !strings.Contains(a.Note, "web01:/p/abc.1/alp.txt") {
		t.Errorf("artifact = %+v", a)
	}
}
