package resolver

import (
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
	"time"
)

const digTimeout = 4 * time.Second

type dnsAnswer struct {
	cnames []string
	ip     string
}

func runDig(ctx context.Context, name, recordType string) ([]string, error) {
	queryContext, cancel := context.WithTimeout(ctx, digTimeout)
	defer cancel()

	cmd := exec.CommandContext(queryContext, "dig", name, recordType, "+short")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("dig command failed for %s (%s): %w", name, recordType, err)
	}

	raw := strings.TrimSpace(string(output))
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

func lookup(ctx context.Context, name, recordType string) (dnsAnswer, error) {
	lines, err := runDig(ctx, name, recordType)
	if err != nil {
		return dnsAnswer{}, err
	}

	var answer dnsAnswer
	for _, line := range lines {
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

	return answer, nil
}
