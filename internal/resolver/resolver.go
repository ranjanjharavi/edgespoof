package resolver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type Result struct {
	Domain      string
	CNAMEChain  []string
	StagingSwap string
	TargetCNAME string
	IPv4        string
	IPv6        string
	StagingMode bool
}

type addressResults struct {
	ipv4 dnsAnswer
	ipv6 dnsAnswer
}

func Resolve(ctx context.Context, domain string, staging bool, v4Only bool) (*Result, error) {
	domain = strings.TrimSpace(domain)
	domain = strings.ToLower(domain)
	domain = strings.TrimSuffix(domain, ".")
	result := &Result{
		Domain:      domain,
		StagingMode: staging,
	}

	answers, lookupErr := lookupAddresses(ctx, domain, domain, v4Only)
	result.CNAMEChain = answers.ipv4.cnames
	if len(result.CNAMEChain) == 0 {
		result.CNAMEChain = answers.ipv6.cnames
	}

	ipv4Target := domain
	ipv6Target := domain
	if staging {
		stagingIPv4, stagingIPv6, found := stagingTargets(answers)
		if !found {
			err := fmt.Errorf("no supported Akamai CNAME found for %s", domain)
			return nil, errors.Join(err, lookupErr)
		}

		ipv4Target = stagingIPv4
		ipv6Target = stagingIPv6
		answers, lookupErr = lookupAddresses(ctx, ipv4Target, ipv6Target, v4Only)
	}

	if answers.ipv4.ip == "" && answers.ipv6.ip == "" {
		if lookupErr != nil {
			return nil, lookupErr
		}

		return nil, fmt.Errorf("no valid A or AAAA records found for %s", domain)
	}

	result.IPv4 = answers.ipv4.ip
	result.IPv6 = answers.ipv6.ip
	primary := answers.ipv4
	target := ipv4Target
	if primary.ip == "" {
		primary = answers.ipv6
		target = ipv6Target
	}
	if staging {
		result.StagingSwap = target
	}
	result.TargetCNAME = strings.TrimSuffix(target, ".")
	if len(primary.cnames) > 0 {
		terminal := primary.cnames[len(primary.cnames)-1]
		result.TargetCNAME = strings.TrimSuffix(terminal, ".")
	}

	return result, nil
}

func lookupAddresses(ctx context.Context, ipv4Name string, ipv6Name string, v4Only bool) (addressResults, error) {
	var answers addressResults
	var errV4 error
	var errV6 error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		answers.ipv4, errV4 = lookup(ctx, ipv4Name, "A")
	}()

	if !v4Only {
		wg.Add(1)
		go func() {
			defer wg.Done()
			answers.ipv6, errV6 = lookup(ctx, ipv6Name, "AAAA")
		}()
	}
	wg.Wait()

	return answers, errors.Join(errV4, errV6)
}

func stagingTargets(answers addressResults) (string, string, bool) {
	ipv4, foundIPv4 := findAndRewriteStaging(answers.ipv4.cnames)
	ipv6, foundIPv6 := findAndRewriteStaging(answers.ipv6.cnames)

	if !foundIPv4 && !foundIPv6 {
		return "", "", false
	}
	if !foundIPv4 {
		ipv4 = ipv6
	}
	if !foundIPv6 {
		ipv6 = ipv4
	}

	return ipv4, ipv6, true
}

func findAndRewriteStaging(cnames []string) (string, bool) {
	rules := []struct {
		production string
		staging    string
	}{
		{
			production: ".edgekey.net.",
			staging:    ".edgekey-staging.net.",
		},
		{
			production: ".edgesuite.net.",
			staging:    ".edgesuite-staging.net.",
		},
		{
			production: ".akamaiedge.net.",
			staging:    ".akamaiedge-staging.net.",
		},
		{
			production: ".akamai.net.",
			staging:    ".akamai-staging.net.",
		},
	}

	for _, cname := range cnames {
		lower := strings.ToLower(cname)
		for _, rule := range rules {
			prefix, found := strings.CutSuffix(lower, rule.production)
			if found {
				return prefix + rule.staging, true
			}
		}
	}
	return "", false
}
