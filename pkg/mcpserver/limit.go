package mcpserver

import (
	"fmt"
	"strings"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

const (
	DefaultMaxFileBytes     = 8 * 1024
	DefaultMaxResponseBytes = 48 * 1024
)

// Limits bound the artifact content in a response; zero means the default.
type Limits struct {
	MaxFileBytes     int
	MaxResponseBytes int
}

func (l Limits) withDefaults() Limits {
	if l.MaxFileBytes <= 0 {
		l.MaxFileBytes = DefaultMaxFileBytes
	}
	if l.MaxResponseBytes <= 0 {
		l.MaxResponseBytes = DefaultMaxResponseBytes
	}
	return l
}

// limitArtifacts returns a copy whose contents fit the limits. Clients differ in what they do with a large response
// (Claude Code rejects all of it), so the cut is made here, on whole lines from the top because the tools list the most important lines first.
func limitArtifacts(r *cmd.AfterBenchResult, l Limits) *cmd.AfterBenchResult {
	if r == nil {
		return nil
	}
	l = l.withDefaults()
	out := *r
	out.Artifacts = make([]cmd.Artifact, len(r.Artifacts))
	remaining := l.MaxResponseBytes
	for i, a := range r.Artifacts {
		budget := min(l.MaxFileBytes, remaining)
		if len(a.Content) > budget {
			a.Content = headLines(a.Content, budget)
			a.Note = limitNote(a, len(a.Content), budget == 0 && remaining == 0)
		}
		remaining -= len(a.Content)
		out.Artifacts[i] = a
	}
	return &out
}

func headLines(s string, budget int) string {
	end := 0
	for pos := 0; pos < len(s); {
		next := strings.IndexByte(s[pos:], '\n')
		if next < 0 {
			break
		}
		if pos+next+1 > budget {
			break
		}
		pos += next + 1
		end = pos
	}
	return s[:end]
}

func limitNote(a cmd.Artifact, kept int, omitted bool) string {
	where := fmt.Sprintf("%s:%s", a.Host, a.Path)
	if omitted {
		return fmt.Sprintf("omitted: the response size limit is reached; original size %d bytes, full output on the server: %s", a.Size, where)
	}
	return fmt.Sprintf("truncated: kept %d of %d bytes (whole lines from the top); full output on the server: %s", kept, a.Size, where)
}
