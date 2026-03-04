package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"gonesis/x/config"
)

// WritePID writes the current process PID to ~/.gonesis/gonesis.pid.
func WritePID() error {
	path, err := config.GlobalFilePath("gonesis.pid")
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

// RemovePID removes the PID file. It is idempotent.
func RemovePID() {
	path, err := config.GlobalFilePath("gonesis.pid")
	if err != nil {
		return
	}
	os.Remove(path)
}

// ReadPID reads and returns the PID from the PID file.
func ReadPID() (int, error) {
	path, err := config.GlobalFilePath("gonesis.pid")
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}
	return pid, nil
}

// IsRunning checks whether the daemon process is alive.
func IsRunning() bool {
	pid, err := ReadPID()
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
