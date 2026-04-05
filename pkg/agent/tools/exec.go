package tools

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"

	"wildgecu/pkg/provider/tool"
)

// ExecTools returns execution tools (bash, node) bound to workDir.
func ExecTools(workDir string) []tool.Tool {
	return []tool.Tool{newBashTool(workDir), newNodeTool(workDir)}
}

// --- bash ---

type bashInput struct {
	Command string `json:"command" description:"The bash command to execute"`
}

type bashOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

func newBashTool(workDir string) tool.Tool {
	return tool.NewTool("bash", "Execute a bash command and return its output",
		func(ctx context.Context, in bashInput) (bashOutput, error) {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
			cmd.Dir = workDir

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			exitCode := 0
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					exitCode = exitErr.ExitCode()
				} else {
					return bashOutput{}, err
				}
			}

			return bashOutput{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				ExitCode: exitCode,
			}, nil
		},
	)
}

// --- node ---

type nodeInput struct {
	Script string `json:"script" description:"The Node.js script to execute"`
}

type nodeOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

func newNodeTool(workDir string) tool.Tool {
	return tool.NewTool("node", "Execute a Node.js script and return its output",
		func(ctx context.Context, in nodeInput) (nodeOutput, error) {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "node", "-e", in.Script)
			cmd.Dir = workDir

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			exitCode := 0
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					exitCode = exitErr.ExitCode()
				} else {
					return nodeOutput{}, err
				}
			}

			return nodeOutput{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				ExitCode: exitCode,
			}, nil
		},
	)
}
