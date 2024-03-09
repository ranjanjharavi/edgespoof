package system

import (
	"os/exec"
	"runtime"
)

// Flushes the local OS DNS cache so /etc/hosts reloads immediately
func FlushDNSCache() error {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("dscacheutil", "-flushcache").Run()
		return exec.Command("killall", "-HUP", "mDNSResponder").Run()

	case "linux":
		if _, err := exec.LookPath("resolvectl"); err == nil {
			return exec.Command("resolvectl", "flush-caches").Run()
		}

		if _, err := exec.LookPath("systemd-resolve"); err == nil {
			return exec.Command("systemd-resolve", "--flush-caches").Run()
		}
		return nil

	default:
		return nil
	}
}
