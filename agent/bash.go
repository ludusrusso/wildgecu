package agent

import (
	"bytes"
	"context"
	"os/exec"
	"time"

	"gonesis/provider/tool"
)

// BashInput is the input for the bash tool.
type BashInput struct {
	Command string `json:"command" description:"The bash command to execute"`
}

// BashOutput is the output for the bash tool.
type BashOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

var bashTool = tool.NewTool("bash", "Execute a bash command and return its output",
	func(ctx context.Context, in BashInput) (BashOutput, error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return BashOutput{}, err
			}
		}

		return BashOutput{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitCode,
		}, nil
	},
)
