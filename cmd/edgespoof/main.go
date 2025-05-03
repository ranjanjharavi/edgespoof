package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/yupcode/edgespoof/internal/hosts"
	"github.com/yupcode/edgespoof/internal/resolver"
)

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
	}
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s✖ Error:%s %v\n", colorRed, colorReset, err)
		os.Exit(1)
	}
}

func printResolution(res *resolver.Result) {
	fmt.Printf("\n%sDomain%s       : %s%s%s\n", colorCyan, colorReset, colorBold, res.Domain, colorReset)

	for _, cname := range res.CNAMEChain {
		fmt.Printf("%sCNAME%s        : -> %s\n", colorCyan, colorReset, cname)
	}

	if res.StagingSwap != "" {
		fmt.Printf("%sStaging Swap%s : -> %s%s%s\n", colorYellow, colorReset, colorYellow, res.StagingSwap, colorReset)
	}

	fmt.Printf("%sTarget Edge%s  : %s%s%s\n", colorCyan, colorReset, colorGreen, res.TargetCNAME, colorReset)

	if res.IPv4 != "" {
		fmt.Printf("%sIPv4 (A)%s     : %s%s%s\n", colorCyan, colorReset, colorBold, res.IPv4, colorReset)
	} else {
		fmt.Printf("%sIPv4 (A)%s     : %snone%s\n", colorCyan, colorReset, colorDim, colorReset)
	}

	if res.IPv6 != "" {
		fmt.Printf("%sIPv6 (AAAA)%s  : %s%s%s\n", colorCyan, colorReset, colorBold, res.IPv6, colorReset)
	} else {
		fmt.Printf("%sIPv6 (AAAA)%s  : %snone%s\n", colorCyan, colorReset, colorDim, colorReset)
	}
	fmt.Println()
}

func printEntries(entries []hosts.Entry) {
	fmt.Printf("\n%s%-28s %-16s %-30s %-10s %-34s%s\n", colorBold, "DOMAIN", "IPv4 (A)", "IPv6 (AAAA)", "MODE", "EDGE TARGET", colorReset)
	fmt.Printf("%s%s%s\n", colorDim, strings.Repeat("-", 122), colorReset)

	for _, e := range entries {
		v4 := e.IPv4
		if v4 == "" {
			v4 = "-"
		}
		v6 := e.IPv6
		if v6 == "" {
			v6 = "-"
		}

		modePlain := fmt.Sprintf("%-10s", "edge")
		modeColored := colorGreen + modePlain + colorReset
		if e.Staging {
			modePlain = fmt.Sprintf("%-10s", "staging")
			modeColored = colorYellow + modePlain + colorReset
		}

		fmt.Printf("%-28s %-16s %-30s %s %-34s\n", e.Domain, v4, v6, modeColored, e.TargetCNAME)
	}
	fmt.Println()
}

func printUsage() {
	fmt.Printf(`%sedgespoof%s — Atomic CDN CNAME Resolver & Dual-Stack /etc/hosts Switcher

%sUSAGE:%s
  edgespoof resolve <domain> [-s|--staging] [--v4-only]   Dry-run CNAME & IP resolution
  sudo edgespoof set <domain> [-s|--staging] [--v4-only]  Pin domain to edge/staging IPs in /etc/hosts
  sudo edgespoof unset <domain>                           Remove domain from /etc/hosts
  edgespoof list                                          List all currently pinned domains
  edgespoof status                                        Print compact status for shell prompt hooks
  sudo edgespoof clear                                    Remove all edgespoof entries from /etc/hosts

%sEXAMPLES:%s
  edgespoof resolve example.com
  sudo edgespoof set example.com
  sudo edgespoof set example.com -s
  sudo edgespoof unset example.com
`, colorBold, colorReset, colorCyan, colorReset, colorCyan, colorReset)
}
