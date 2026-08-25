//go:build windows

package aihealth

import (
	"fmt"
	"os/exec"
	"syscall"
)

const (
	detachedProcess = 0x00000008
	// CREATE_NEW_PROCESS_GROUP must be 0x200, not 0x2. The value 0x2 is
	// DEBUG_ONLY_THIS_PROCESS: it makes the backend the child's debugger, and
	// since no debug events are pumped the launched model process stalls and
	// never becomes reachable (verified by launch-flag repro).
	createNewProcessGroup = 0x00000200
)

// defaultLaunch fire-and-forgets cmd via "cmd /c" in a new process group with
// no inherited console window. It does not Wait and stores no PID; the process
// lifecycle is intentionally unmanaged (spec: 进程生命周期不被托管).
func defaultLaunch(cmd string) error {
	c := exec.Command("cmd", "/c", cmd) //nolint:gosec // G204 — start_command is a user-configured local launch string; executing it is the intended behavior
	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: detachedProcess | createNewProcessGroup,
	}
	if err := c.Start(); err != nil {
		return fmt.Errorf("launch %q: %w", cmd, err)
	}
	if c.Process != nil {
		_ = c.Process.Release()
	}
	return nil
}
