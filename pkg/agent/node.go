package agent

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"

	"wildgecu/pkg/provider/tool"
)

// NodeInput is the input for the node tool.
type NodeInput struct {
	Script string `json:"script" description:"The Node.js script to execute"`
}

// NodeOutput is the output for the node tool.
type NodeOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

func newNodeTool(homeDir string) tool.Tool {
	return tool.NewTool("node", "Execute a Node.js script and return its output",
		func(ctx context.Context, in NodeInput) (NodeOutput, error) {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "node", "-e", in.Script)
			cmd.Dir = homeDir

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
					return NodeOutput{}, err
				}
			}

			return NodeOutput{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				ExitCode: exitCode,
			}, nil
		},
	)
}
