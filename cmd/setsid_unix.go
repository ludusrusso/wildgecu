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

	cmd := exec.Command(exe, "start", "--daemon")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	fmt.Printf("Daemon started (pid %d).\n", cmd.Process.Pid)
	cmd.Process.Release()
	return nil
}
