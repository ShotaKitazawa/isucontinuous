package mcpserver

import (
	"context"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type progressReporter struct {
	ctx     context.Context
	session *mcp.ServerSession
	token   any

	mu    sync.Mutex
	total float64
	count float64
}

func newProgressReporter(ctx context.Context, session *mcp.ServerSession, token any) *progressReporter {
	return &progressReporter{ctx: ctx, session: session, token: token}
}

// Open counts a start and a finish event per task so that progress never stalls between them.
func (r *progressReporter) Open(hosts []string, tasks int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.total = float64(len(hosts) * tasks * 2)
	return nil
}

func (r *progressReporter) Close() {}

func (r *progressReporter) TaskStarted(host, task string) {
	r.notify(fmt.Sprintf("[%s] %s: started", host, task))
}

func (r *progressReporter) TaskDone(host, task string) {
	r.notify(fmt.Sprintf("[%s] %s: done", host, task))
}

func (r *progressReporter) TaskFailed(host, task string, err error) {
	r.notify(fmt.Sprintf("[%s] %s: failed: %v", host, task, err))
}

func (r *progressReporter) notify(message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.count++
	_ = r.session.NotifyProgress(r.ctx, &mcp.ProgressNotificationParams{
		ProgressToken: r.token,
		Progress:      r.count,
		Total:         r.total,
		Message:       message,
	})
}

// silentReporter is used without a progress token: the default reporter draws bars on stdout, which would corrupt the stdio protocol.
type silentReporter struct{}

func (silentReporter) Open(hosts []string, tasks int) error  { return nil }
func (silentReporter) Close()                                {}
func (silentReporter) TaskStarted(host, task string)         {}
func (silentReporter) TaskDone(host, task string)            {}
func (silentReporter) TaskFailed(host, task string, e error) {}
