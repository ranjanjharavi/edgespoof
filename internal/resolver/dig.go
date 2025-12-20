package resolver

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
	"time"
)

const (
	digTimeout = 5 * time.Second

	// dig exit status when no DNS server replies
	digNoReply = 9
)

type dnsAnswer struct {
	cnames []string
	ip     string
}

func checkDig() error {
	if _, err := exec.LookPath("dig"); err != nil {
		return errors.New("dig not found in $PATH. Install dnsutils (Debian, Ubuntu), bind-utils (Fedora, RHEL), or bind-tools (Alpine, Arch)")
	}
	return nil
}

func runDig(ctx context.Context, name, recordType string) (string, error) {
	queryContext, cancel := context.WithTimeout(ctx, digTimeout)
	defer cancel()

	// -q and -t stop dig from reading the name as a server or option
	cmd := exec.CommandContext(queryContext, "dig", "+short", "+time=2", "+tries=2", "-t", recordType, "-q", name)
	// Stop waiting on output pipes held open by leftover child processes
	cmd.WaitDelay = time.Second
	output, err := cmd.Output()
	if err != nil {
		return "", digError(queryContext, name, recordType, err)
	}

	return string(output), nil
}

func digError(ctx context.Context, name, recordType string, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%s lookup for %s timed out after %s", recordType, name, digTimeout)
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return fmt.Errorf("%s lookup for %s failed: %w", recordType, name, err)
	}
	if exitErr.ExitCode() == digNoReply {
		return fmt.Errorf("%s lookup for %s timed out. No DNS server replied", recordType, name)
	}

	detail := strings.TrimSpace(string(exitErr.Stderr))
	if detail == "" {
		return fmt.Errorf("%s lookup for %s failed: %w", recordType, name, err)
	}

	return fmt.Errorf("%s lookup for %s failed: %w (%s)", recordType, name, err, detail)
}

func lookup(ctx context.Context, name, recordType string) (dnsAnswer, error) {
	output, err := runDig(ctx, name, recordType)
	if err != nil {
		return dnsAnswer{}, err
	}

	return parseAnswer(output, recordType), nil
}

// CNAME lines end with '.'. First valid address of the record family wins
func parseAnswer(output, recordType string) dnsAnswer {
	var answer dnsAnswer
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ".") {
			answer.cnames = append(answer.cnames, line)
			continue
		}
		if answer.ip != "" {
			continue
		}

		address, err := netip.ParseAddr(line)
		if err != nil {
			continue
		}
		if address.Zone() != "" {
			continue
		}

		address = address.Unmap()
		if recordType == "A" && !address.Is4() {
			continue
		}
		if recordType == "AAAA" && !address.Is6() {
			continue
		}

		answer.ip = address.String()
	}

	return answer
}
