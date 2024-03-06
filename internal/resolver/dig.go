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

const digTimeout = 4 * time.Second

// To hold the complete resolution chain and dual-stack IPs
type Result struct {
	Domain      string
	CNAMEChain  []string
	StagingSwap string
	TargetCNAME string
	IPv4        string
	IPv6        string
	StagingMode bool
}

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

// Scans CNAME hops in order and swaps the first matching CDN suffix
func findAndRewriteStaging(cnames []string) (string, bool) {
	rules := []struct {
		prod    string
		staging string
	}{
		{".edgekey.net.", ".edgekey-staging.net."},
		{".edgesuite.net.", ".edgesuite-staging.net."},
		{".akamaiedge.net.", ".akamaiedge-staging.net."},
		{".akamai.net.", ".akamai-staging.net."},
	}

	for _, cname := range cnames {
		lower := strings.ToLower(cname)

		for _, rule := range rules {
			if strings.HasSuffix(lower, rule.prod) {
				// Strip the production suffix and append the staging suffix
				swapped := strings.TrimSuffix(lower, rule.prod) + rule.staging
				return swapped, true
			}
		}
	}

	return "", false
}

// Traces the CNAME chain and fetches the first A and AAAA records concurrently
func Resolve(ctx context.Context, domain string, staging bool, v4Only bool) (*Result, error) {
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	res := &Result{
		Domain:      domain,
		StagingMode: staging,
	}

	// Trace Initial CNAME chain
	lines, err := runDig(ctx, domain, "A")
	if err != nil {
		return nil, err
	}

	for _, line := range lines {
		if strings.HasSuffix(line, ".") {
			res.CNAMEChain = append(res.CNAMEChain, line)
		}
	}

	// Select TargetCNAME: Staging Rewrite vs. Production Terminal CNAME
	if staging {
		stagingHost, ok := findAndRewriteStaging(res.CNAMEChain)
		if !ok {
			return nil, fmt.Errorf("no Akamai CNAME (.edgekey.net, .edgesuite.net, .akamaiedge.net) found in chain for %s", domain)
		}

		res.StagingSwap = stagingHost

		// Run secondary dig on the rewritten staging CNAME to find the staging edge node
		stagingLines, err := runDig(ctx, stagingHost, "A")
		if err != nil {
			return nil, fmt.Errorf("staging dig failed for %s: %w", stagingHost, err)
		}

		var terminalStaging string
		for _, line := range stagingLines {
			if strings.HasSuffix(line, ".") {
				terminalStaging = line
			}
		}

		if terminalStaging != "" {
			res.TargetCNAME = strings.TrimSuffix(terminalStaging, ".")
		} else {
			res.TargetCNAME = strings.TrimSuffix(stagingHost, ".")
		}
	} else {
		if len(res.CNAMEChain) > 0 {
			res.TargetCNAME = strings.TrimSuffix(res.CNAMEChain[len(res.CNAMEChain)-1], ".")
		} else {
			res.TargetCNAME = domain
		}
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
