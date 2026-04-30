package tools

import (
	"context"
	"time"

	"github.com/ludusrusso/wildgecu/pkg/exec/bounded"
	"github.com/ludusrusso/wildgecu/pkg/provider/tool"
)

// ExecConfig configures the execution tools (bash, node). Zero values fall
// back to the bounded package's defaults.
type ExecConfig struct {
	// MaxTimeoutSeconds is the hard ceiling on the per-call
	// timeout_seconds argument. Zero = use bounded.DefaultMaxTimeout.
	MaxTimeoutSeconds int
	// HeadBytes is the size of the captured head window for
	// stdout/stderr. Zero = use bounded.DefaultHeadBytes.
	HeadBytes int
	// TailBytes is the size of the captured tail window for
	// stdout/stderr. Zero = use bounded.DefaultTailBytes.
	TailBytes int
}

// ExecTools returns execution tools (bash, node) bound to workDir.
func ExecTools(workDir string, cfg ExecConfig) []tool.Tool {
	return []tool.Tool{newBashTool(workDir, cfg), newNodeTool(workDir, cfg)}
}

// boundedOpts builds bounded.Opts from cfg and a per-call timeout_seconds. A
// zero or negative timeoutSeconds picks the bounded.DefaultTimeout.
func (cfg ExecConfig) boundedOpts(workDir string, timeoutSeconds int) bounded.Opts {
	opts := bounded.Opts{
		HeadBytes: cfg.HeadBytes,
		TailBytes: cfg.TailBytes,
		WorkDir:   workDir,
	}
	if cfg.MaxTimeoutSeconds > 0 {
		opts.MaxTimeout = time.Duration(cfg.MaxTimeoutSeconds) * time.Second
	}
	if timeoutSeconds > 0 {
		opts.Timeout = time.Duration(timeoutSeconds) * time.Second
	}
	return opts
}

// --- bash ---

type bashInput struct {
	Command        string `json:"command" description:"The bash command to execute"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" description:"Per-call timeout in seconds. Default 30, hard-capped at 600."`
}

type bashOutput struct {
	Stdout           string `json:"stdout"`
	Stderr           string `json:"stderr"`
	ExitCode         int    `json:"exit_code"`
	TimedOut         bool   `json:"timed_out,omitempty"`
	StdoutTotalBytes int    `json:"stdout_total_bytes"`
	StderrTotalBytes int    `json:"stderr_total_bytes"`
}

func newBashTool(workDir string, cfg ExecConfig) tool.Tool {
	return tool.NewTool("bash",
		"Execute a bash command and return its output. Pass timeout_seconds (default 30, hard-capped at 600) to give long-running commands more time.",
		func(ctx context.Context, in bashInput) (bashOutput, error) {
			res, err := bounded.Run(ctx, "bash", []string{"-c", in.Command}, cfg.boundedOpts(workDir, in.TimeoutSeconds))
			if err != nil {
				return bashOutput{}, err
			}
			return bashOutput{
				Stdout:           res.Stdout,
				Stderr:           res.Stderr,
				ExitCode:         res.ExitCode,
				TimedOut:         res.TimedOut,
				StdoutTotalBytes: res.StdoutTotalBytes,
				StderrTotalBytes: res.StderrTotalBytes,
			}, nil
		},
	)
}

// --- node ---

type nodeInput struct {
	Script         string `json:"script" description:"The Node.js script to execute"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" description:"Per-call timeout in seconds. Default 30, hard-capped at 600."`
}

type nodeOutput struct {
	Stdout           string `json:"stdout"`
	Stderr           string `json:"stderr"`
	ExitCode         int    `json:"exit_code"`
	TimedOut         bool   `json:"timed_out,omitempty"`
	StdoutTotalBytes int    `json:"stdout_total_bytes"`
	StderrTotalBytes int    `json:"stderr_total_bytes"`
}

func newNodeTool(workDir string, cfg ExecConfig) tool.Tool {
	return tool.NewTool("node",
		"Execute a Node.js script and return its output. Pass timeout_seconds (default 30, hard-capped at 600) to give long-running scripts more time.",
		func(ctx context.Context, in nodeInput) (nodeOutput, error) {
			res, err := bounded.Run(ctx, "node", []string{"-e", in.Script}, cfg.boundedOpts(workDir, in.TimeoutSeconds))
			if err != nil {
				return nodeOutput{}, err
			}
			return nodeOutput{
				Stdout:           res.Stdout,
				Stderr:           res.Stderr,
				ExitCode:         res.ExitCode,
				TimedOut:         res.TimedOut,
				StdoutTotalBytes: res.StdoutTotalBytes,
				StderrTotalBytes: res.StderrTotalBytes,
			}, nil
		},
	)
}
