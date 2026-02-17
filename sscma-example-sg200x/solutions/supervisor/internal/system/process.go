package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// StopProcessByName stops all processes with the given name.
// signal: 9 (SIGKILL) for immediate termination, 15 (SIGTERM) for graceful shutdown
func StopProcessByName(name string, signal int) error {
	pids, err := FindProcessByName(name)
	if err != nil {
		return err
	}

	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		process.Signal(syscall.Signal(signal))
	}
	return nil
}

// FindProcessByName returns PIDs of all processes with the given name.
func FindProcessByName(name string) ([]int, error) {
	cmd := exec.Command("pidof", name)
	out, err := cmd.Output()
	if err != nil {
		// pidof returns error if no process found
		return nil, nil
	}

	var pids []int
	for _, pidStr := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(pidStr)
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

// IsProcessRunning checks if a process with the given name is running.
func IsProcessRunning(name string) bool {
	pids, _ := FindProcessByName(name)
	return len(pids) > 0
}

// RunService executes a service init script with the given action.
func RunService(service, action string) error {
	return exec.Command("/etc/init.d/"+service, action).Run()
}

// RunServiceAsync executes a service init script asynchronously.
func RunServiceAsync(service, action string) {
	go exec.Command("/etc/init.d/"+service, action).Run()
}

const defaultPowerActionTimeout = 8 * time.Second

// Reboot requests a system reboot.
//
// It tries multiple implementations (systemd, busybox, absolute paths) and uses a timeout
// for each attempt to avoid hanging indefinitely.
func Reboot() error {
	return rebootWithTimeout(defaultPowerActionTimeout)
}

// Poweroff requests a system shutdown/poweroff.
func Poweroff() error {
	return poweroffWithTimeout(defaultPowerActionTimeout)
}

// Suspend requests a system suspend (best-effort; not all images support this).
func Suspend() error {
	return suspendWithTimeout(defaultPowerActionTimeout)
}

func rebootWithTimeout(timeout time.Duration) error {
	// Best-effort flush before reboot.
	_ = runCommandWithTimeout(timeout, "sync")
	// Give the filesystem a moment to settle.
	time.Sleep(250 * time.Millisecond)

	candidates := [][]string{
		{"systemctl", "reboot", "--force", "--force"},
		{"/sbin/reboot", "-f"},
		{"/bin/reboot", "-f"},
		{"/sbin/reboot"},
		{"/bin/reboot"},
		{"reboot", "-f"},
		{"reboot"},
		{"busybox", "reboot", "-f"},
	}
	return runCandidatesWithTimeout("reboot", candidates, timeout)
}

func poweroffWithTimeout(timeout time.Duration) error {
	_ = runCommandWithTimeout(timeout, "sync")
	time.Sleep(250 * time.Millisecond)

	candidates := [][]string{
		{"systemctl", "poweroff", "--force", "--force"},
		{"/sbin/poweroff", "-f"},
		{"/bin/poweroff", "-f"},
		{"/sbin/poweroff"},
		{"/bin/poweroff"},
		{"poweroff", "-f"},
		{"poweroff"},
		{"busybox", "poweroff", "-f"},
	}
	return runCandidatesWithTimeout("poweroff", candidates, timeout)
}

func suspendWithTimeout(timeout time.Duration) error {
	candidates := [][]string{
		{"systemctl", "suspend"},
		{"pm-suspend"},
		{"suspend"},
	}
	return runCandidatesWithTimeout("suspend", candidates, timeout)
}

func runCandidatesWithTimeout(action string, candidates [][]string, timeout time.Duration) error {
	var lastErr error
	for _, c := range candidates {
		if len(c) == 0 {
			continue
		}
		err := runCommandWithTimeout(timeout, c[0], c[1:]...)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	if lastErr == nil {
		return fmt.Errorf("%s: no candidates to run", action)
	}
	return fmt.Errorf("%s: all candidates failed, last error: %w", action, lastErr)
}

func runCommandWithTimeout(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("command timed out after %s: %s %s", timeout, name, strings.Join(args, " "))
	}
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if msg != "" {
		return fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, msg)
	}
	return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
}
