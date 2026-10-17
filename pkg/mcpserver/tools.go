package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

const (
	forceRule    = "Taking over a lock held by someone else can break their work: ask the holder first and use force only after they agree. "
	driftRule    = "ignore_drift overwrites files that were edited directly on the servers: use it only after confirming that overwriting them is acceptable. "
	warnRule     = "The result contains warnings: read them and act on them. "
	artifactRule = "The result lists the output files; long ones are cut to whole lines from the top and marked in Note with the path on the server: to see less noise, narrow the output with after_bench.command (e.g. alp -m, pt-query-digest --limit, pprof -nodecount) instead of reading the full file. "
	hostKey      = "If it fails with a host key mismatch, do not work around it: a human must verify the host first. "
)

type Empty struct{}

type InitInput struct {
	Username  string `json:"username,omitempty" jsonschema:"git user.name for local-repo (default isucontinuous)"`
	Email     string `json:"email,omitempty" jsonschema:"git user.email for local-repo"`
	RemoteURL string `json:"remote_url,omitempty" jsonschema:"git remote URL of the repository to manage"`
	SSHKey    string `json:"ssh_key,omitempty" jsonschema:"path of the deploy key used to access remote_url"`
}

type PushInput struct {
	Branch  string `json:"branch,omitempty" jsonschema:"branch to push to (default master)"`
	Message string `json:"message,omitempty" jsonschema:"commit message"`
}

type SyncInput struct {
	Branch string `json:"branch,omitempty" jsonschema:"branch to synchronize with (default master)"`
}

type DeployInput struct {
	Revision    string `json:"revision,omitempty" jsonschema:"branch name, tag name or commit hash to deploy (default master)"`
	Force       bool   `json:"force,omitempty" jsonschema:"take over host locks held by someone else; ask the holder first"`
	IgnoreDrift bool   `json:"ignore_drift,omitempty" jsonschema:"deploy even if files on hosts differ from the revision deployed last; this overwrites direct edits on the servers"`
}

type ReleaseInput struct {
	Force           bool `json:"force,omitempty" jsonschema:"also release locks held by someone else; ask the holder first"`
	IncludeDisabled bool `json:"include_disabled,omitempty" jsonschema:"also release the locks of disabled hosts"`
}

type DiffInput struct {
	Revision string `json:"revision,omitempty" jsonschema:"revision to compare with (default: the revision each host was deployed with last)"`
	ShowDiff bool   `json:"show_diff,omitempty" jsonschema:"include the content of the differences"`
}

// adapt turns a cmd.Run* call into a tool handler; a failed run still returns what it completed, so the caller can see how far it got.
func adapt[In any, Cfg any, Out any](
	opts Options, build func(In, cmd.ConfigCommon) Cfg, run func(Cfg) (*Out, error),
) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		common := opts.Common
		common.Reporter = silentReporter{}
		if token := req.Params.GetProgressToken(); token != nil {
			common.Reporter = newProgressReporter(ctx, req.Session, token)
		}
		out, err := run(build(in, common))
		var result Out
		if out != nil {
			result = *out
		}
		if err != nil {
			return errorResult(err), result, nil
		}
		return nil, result, nil
	}
}

func registerTools(s *mcp.Server, o Options) {
	r := o.Runners
	mcp.AddTool(s, &mcp.Tool{Name: "init", Description: "Initialize local-repo from the remote repository. " + hostKey + warnRule},
		adapt(o, func(in InitInput, c cmd.ConfigCommon) cmd.ConfigInit {
			return cmd.ConfigInit{ConfigCommon: c, GitUsername: in.Username, GitEmail: in.Email, GitRemoteUrl: in.RemoteURL, SshKey: in.SSHKey}
		}, r.Init))
	mcp.AddTool(s, &mcp.Tool{Name: "setup", Description: "Install the software listed in isucontinuous.yaml on the hosts. Locks the hosts while it runs; if another owner holds a lock it fails, and the holder must release it (ask them, or use release with force). " + hostKey + warnRule},
		adapt(o, func(_ Empty, c cmd.ConfigCommon) cmd.ConfigSetup {
			return cmd.ConfigSetup{ConfigCommon: c}
		}, r.Setup))
	mcp.AddTool(s, &mcp.Tool{Name: "import", Description: "Import the files listed in isucontinuous.yaml from the hosts into local-repo. Locks the hosts while it runs; if another owner holds a lock it fails, and the holder must release it (ask them, or use release with force). " + hostKey + warnRule},
		adapt(o, func(_ Empty, c cmd.ConfigCommon) cmd.ConfigImport {
			return cmd.ConfigImport{ConfigCommon: c}
		}, r.Import))
	mcp.AddTool(s, &mcp.Tool{Name: "push", Description: "Commit local-repo and push it to the remote branch. Changes not pushed are not deployed. " + warnRule},
		adapt(o, func(in PushInput, c cmd.ConfigCommon) cmd.ConfigPush {
			return cmd.ConfigPush{ConfigCommon: c, GitBranch: orDefault(in.Branch, "master"), Message: in.Message}
		}, r.Push))
	mcp.AddTool(s, &mcp.Tool{Name: "sync", Description: "Synchronize local-repo with the remote branch. It refuses to run with uncommitted changes in local-repo. " + warnRule},
		adapt(o, func(in SyncInput, c cmd.ConfigCommon) cmd.ConfigSync {
			return cmd.ConfigSync{ConfigCommon: c, GitBranch: orDefault(in.Branch, "master")}
		}, r.Sync))
	mcp.AddTool(s, &mcp.Tool{Name: "deploy", Description: "Deploy the files of a revision to the hosts. Locks the hosts until afterbench or release. " + forceRule + driftRule + hostKey + warnRule},
		adapt(o, func(in DeployInput, c cmd.ConfigCommon) cmd.ConfigDeploy {
			return cmd.ConfigDeploy{ConfigCommon: c, GitRevision: orDefault(in.Revision, "master"), Force: in.Force, IgnoreDrift: in.IgnoreDrift, SlackToken: o.SlackToken}
		}, r.Deploy))
	mcp.AddTool(s, &mcp.Tool{Name: "profiling", Description: "Run the profiling commands on the hosts and wait for them to finish. Requires the locks taken by deploy. " + hostKey + warnRule},
		adapt(o, func(_ Empty, c cmd.ConfigCommon) cmd.ConfigProfiling {
			return cmd.ConfigProfiling{ConfigCommon: c}
		}, r.Profiling))
	mcp.AddTool(s, &mcp.Tool{Name: "afterbench", Description: "Collect and parse the profile data and notify Slack. Releases the locks on success; on failure the locks are kept. " + artifactRule + hostKey + warnRule},
		adapt(o, func(_ Empty, c cmd.ConfigCommon) cmd.ConfigAfterBench {
			return cmd.ConfigAfterBench{ConfigCommon: c, SlackToken: o.SlackToken}
		}, func(c cmd.ConfigAfterBench) (*cmd.AfterBenchResult, error) {
			out, err := r.AfterBench(c)
			return limitArtifacts(out, o.Limits), err
		}))
	mcp.AddTool(s, &mcp.Tool{Name: "release", Description: "Release the host locks you hold. " + forceRule + warnRule},
		adapt(o, func(in ReleaseInput, c cmd.ConfigCommon) cmd.ConfigRelease {
			return cmd.ConfigRelease{ConfigCommon: c, Force: in.Force, IncludeDisabled: in.IncludeDisabled}
		}, r.Release))
	mcp.AddTool(s, &mcp.Tool{Name: "status", Description: "Show the lock holder and the revision deployed last of each host. " + hostKey + warnRule},
		adapt(o, func(_ Empty, c cmd.ConfigCommon) cmd.ConfigStatus {
			return cmd.ConfigStatus{ConfigCommon: c}
		}, r.Status))
	mcp.AddTool(s, &mcp.Tool{Name: "diff", Description: "Show files on the hosts whose content differs from a revision. " + hostKey + warnRule},
		adapt(o, func(in DiffInput, c cmd.ConfigCommon) cmd.ConfigDiff {
			return cmd.ConfigDiff{ConfigCommon: c, GitRevision: in.Revision, ShowDiff: in.ShowDiff}
		}, r.Diff))
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
