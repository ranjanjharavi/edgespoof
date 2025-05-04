package system

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func FlushDNSCache() error {
	switch runtime.GOOS {
	case "darwin":
		cacheErr := runFlushCommand("dscacheutil", "-flushcache")
		responderErr := runFlushCommand("killall", "-HUP", "mDNSResponder")
		return errors.Join(cacheErr, responderErr)

	case "linux":
		if path, err := exec.LookPath("resolvectl"); err == nil {
			return runFlushCommand(path, "flush-caches")
		}
		if path, err := exec.LookPath("systemd-resolve"); err == nil {
			return runFlushCommand(path, "--flush-caches")
		}

		return errors.New("no supported DNS cache command found (resolvectl or systemd-resolve)")

	default:
		return fmt.Errorf("DNS cache flushing is not supported on %s", runtime.GOOS)
	}
}

func runFlushCommand(name string, args ...string) error {
	output, err := exec.Command(name, args...).CombinedOutput()
	if err == nil {
		return nil
	}

	detail := strings.TrimSpace(string(output))
	if detail != "" {
		return fmt.Errorf("%s: %w (%s)", name, err, detail)
	}

	return fmt.Errorf("%s: %w", name, err)
}
