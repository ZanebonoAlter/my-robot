//go:build !windows

package aihealth

import (
	"fmt"
	"os/exec"
	"syscall"
)

// defaultLaunch fire-and-forgets cmd in a new session, detached from the
// backend process group. It does not Wait for the process to exit and stores
// no PID; the process lifecycle is intentionally unmanaged (spec: 进程生命周期
// 不被托管 — 不记 PID、不杀进程、不崩溃重启). Re-launching is naturally
// suppressed because an already-reachable provider is never launched.
func defaultLaunch(cmd string) error {
	c := exec.Command("sh", "-c", cmd) //nolint:gosec // G204 — start_command is a user-configured local launch string; executing it is the intended behavior
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := c.Start(); err != nil {
		return fmt.Errorf("launch %q: %w", cmd, err)
	}
	// Release detaches the child so the Go runtime neither reaps nor Waits it.
	if c.Process != nil {
		_ = c.Process.Release()
	}
	return nil
}
