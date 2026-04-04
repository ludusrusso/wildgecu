//go:build !windows

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func reExecDetached() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	args := []string{"start", "--daemon"}
	if homeFlag != "" {
		args = append(args, "--home", homeFlag)
	}

	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	fmt.Printf("Daemon started (pid %d).\n", cmd.Process.Pid)
	_ = cmd.Process.Release()
	return nil
}
