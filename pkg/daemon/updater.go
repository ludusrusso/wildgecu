//go:build !windows

package daemon

import (
	"fmt"
	"net/http"
	"os"
	"syscall"

	"github.com/minio/selfupdate"
)

// SelfUpdate downloads the binary from binaryURL and replaces the current
// executable. On success it sends SIGUSR1 to trigger a re-exec.
func SelfUpdate(binaryURL string) error {
	resp, err := http.Get(binaryURL)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download update: status %s", resp.Status)
	}

	if err := selfupdate.Apply(resp.Body, selfupdate.Options{}); err != nil {
		return fmt.Errorf("apply update: %w", err)
	}

	// Signal ourselves to re-exec with the new binary.
	return syscall.Kill(os.Getpid(), syscall.SIGUSR1)
}
