package resolver

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const digTimeout = 4 * time.Second

func runDig(ctx context.Context, name, qtype string) ([]string, error) {
	// Derived context that automatically cancels after 4 seconds preventing CLI exec when DNS servers are not reachable
	c, cancel := context.WithTimeout(ctx, digTimeout)
	defer cancel()

	// CommandContext kills the subprocess if the context deadline expires
	cmd := exec.CommandContext(c, "dig", name, qtype, "+short")
	out, err := cmd.Output()

	if err != nil {
		return nil, fmt.Errorf("dig command failed for %s (%s): %w", name, qtype, err)
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	return lines, nil
}

// Runs dig command with hostname and separates the CNAME hops from the IP records
func TraceChain(ctx context.Context, domain string) (cnames []string, terminalCNAME string, err error) {
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")

	lines, err := runDig(ctx, domain, "A")
	if err != nil {
		return nil, "", err
	}

	for _, line := range lines {
		if strings.HasSuffix(line, ".") {
			cnames = append(cnames, line)
		}
	}

	if len(cnames) > 0 {
		terminalCNAME = strings.TrimSuffix(cnames[len(cnames)-1], ".")
	} else {
		terminalCNAME = domain
	}

	return cnames, terminalCNAME, nil
}
