package resolver

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// To hold the complete resolution chain and dual-stack IPs
type Result struct {
	Domain      string
	CNAMEChain  []string
	TargetCNAME string
	IPv4        string
	IPv6        string
}

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

// resolveFirstIP queries `dig <target> <recordType> +short` and returns the first valid IP
func resolveFirstIP(ctx context.Context, target, recordType string) (string, error) {
	lines, err := runDig(ctx, target, recordType)
	if err != nil {
		return "", err
	}

	for _, line := range lines {
		// Skip CNAME hops (which end with a dot)
		if strings.HasSuffix(line, ".") {
			continue
		}

		ip := net.ParseIP(line)
		if ip == nil {
			continue
		}

		// Strictly match IPv4 for "A" queries
		if recordType == "A" && ip.To4() != nil {
			return ip.String(), nil
		}

		// Strictly match IPv6 for "AAAA" queries
		if recordType == "AAAA" && ip.To4() == nil && ip.To16() != nil {
			return ip.String(), nil
		}
	}

	return "", nil
}

// Traces the CNAME chain and fetches the first A and AAAA records concurrently
func Resolve(ctx context.Context, domain string, v4Only bool) (*Result, error) {
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	res := &Result{Domain: domain}

	// Trace CNAME chain
	lines, err := runDig(ctx, domain, "A")
	if err != nil {
		return nil, err
	}

	for _, line := range lines {
		if strings.HasSuffix(line, ".") {
			res.CNAMEChain = append(res.CNAMEChain, line)
		}
	}

	if len(res.CNAMEChain) > 0 {
		CNAMEChainIndex := len(res.CNAMEChain) - 1
		res.TargetCNAME = strings.TrimSuffix(res.CNAMEChain[CNAMEChainIndex], ".")
	} else {
		res.TargetCNAME = domain
	}

	// Concurrent Dual-Stack Lookup on TargetCNAME
	var wg sync.WaitGroup
	var errV4, errV6 error

	// Goroutine: Always fetch IPv4 (A)
	wg.Add(1)
	go func() {
		defer wg.Done()
		res.IPv4, errV4 = resolveFirstIP(ctx, res.TargetCNAME, "A")
	}()

	// Goroutine: Fetch IPv6 (AAAA) unless --v4-only is set
	if !v4Only {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res.IPv6, errV6 = resolveFirstIP(ctx, res.TargetCNAME, "AAAA")
		}()
	}

	// Block until both dig subprocesses finish
	wg.Wait()

	// Only fail if both lookups threw an execution error
	if errV4 != nil && errV6 != nil {
		return nil, fmt.Errorf("dual-stack lookup failed (v4: %v, v6: %v)", errV4, errV6)
	}

	if res.IPv4 == "" && res.IPv6 == "" {
		return nil, fmt.Errorf("no valid A or AAAA records found for %s", res.TargetCNAME)
	}

	return res, nil
}
