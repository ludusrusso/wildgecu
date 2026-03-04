//go:build windows

package cmd

import "fmt"

func reExecDetached() error {
	return fmt.Errorf("daemon mode not supported on Windows, use --system")
}
