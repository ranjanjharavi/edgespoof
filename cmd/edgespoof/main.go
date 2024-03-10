package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yupcode/edgespoof/internal/hosts"
	"github.com/yupcode/edgespoof/internal/resolver"
	"github.com/yupcode/edgespoof/internal/system"
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
		os.Exit(1)
	}

	subcmd := strings.ToLower(os.Args[1])
	customHostsPath := os.Getenv("EDGESPOOF_HOSTS_PATH")
	mgr := hosts.NewManager(customHostsPath)

	switch subcmd {
	case "resolve", "set":
		domain, isStaging, v4Only := parseDomainAndFlags(subcmd, os.Args[2:])

		if subcmd == "set" && customHostsPath == "" {
			requireRoot()
		}

		res, err := resolver.Resolve(context.Background(), domain, isStaging, v4Only)
		if err != nil {
			fatalf("%v", err)
		}

		printResolution(res)

		if subcmd == "set" {
			err := mgr.Upsert(hosts.Entry{
				Domain:      res.Domain,
				IPv4:        res.IPv4,
				IPv6:        res.IPv6,
				TargetCNAME: res.TargetCNAME,
				Staging:     res.StagingMode,
			})
			if err != nil {
				fatalf("failed to update hosts file: %v", err)
			}
			_ = system.FlushDNSCache()
			fmt.Printf("%s✔%s Updated hosts file & flushed OS DNS cache\n", colorGreen, colorReset)
		}

	case "unset", "rm":
		if len(os.Args) < 3 {
			fatalf("missing domain argument. Usage: edgespoof unset <domain>")
		}
		if customHostsPath == "" {
			requireRoot()
		}

		domain := os.Args[2]
		removed, err := mgr.Remove(domain)
		if err != nil {
			fatalf("failed to update hosts file: %v", err)
		}
		if !removed {
			fmt.Printf("%s!%s Domain %s was not found in the active edgespoof block.\n", colorYellow, colorReset, domain)
			return
		}
		_ = system.FlushDNSCache()
		fmt.Printf("%s✔%s Removed %s from hosts file & flushed OS DNS cache\n", colorGreen, colorReset, domain)

	case "list", "ls":
		entries, err := mgr.List()
		if err != nil {
			fatalf("failed to read hosts file: %v", err)
		}
		if len(entries) == 0 {
			fmt.Printf("%sNo active edgespoof entries in hosts file.%s\n", colorDim, colorReset)
			return
		}
		printEntries(entries)

	case "status":
		entries, err := mgr.List()
		if err != nil || len(entries) == 0 {
			return
		}
		fmt.Printf("[SPOOF:%d]\n", len(entries))

	case "clear":
		if customHostsPath == "" {
			requireRoot()
		}
		if err := mgr.Clear(); err != nil {
			fatalf("failed to clear hosts block: %v", err)
		}
		_ = system.FlushDNSCache()
		fmt.Printf("%s✔%s Cleared all edgespoof entries & flushed OS DNS cache\n", colorGreen, colorReset)

	case "help", "-h", "--help":
		printUsage()

	default:
		fatalf("unknown command %q. Run 'edgespoof help' for usage.", subcmd)
	}
}

// Allow flags (-s, --staging, --v4-only) to appear BEFORE or AFTER the domain
func parseDomainAndFlags(subcmd string, args []string) (domain string, isStaging bool, v4Only bool) {
	fs := flag.NewFlagSet(subcmd, flag.ExitOnError)
	stagingLong := fs.Bool("staging", false, "Rewrite CDN CNAME to staging network")
	stagingShort := fs.Bool("s", false, "Shorthand for --staging")
	v4 := fs.Bool("v4-only", false, "Resolve IPv4 (A) record only")

	var flagArgs []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
		} else if domain == "" {
			domain = arg
		}
	}

	_ = fs.Parse(flagArgs)

	if domain == "" {
		fatalf("missing domain argument. Usage: edgespoof %s <domain> [-s|--staging] [--v4-only]", subcmd)
	}

	return domain, (*stagingLong || *stagingShort), *v4
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

func requireRoot() {
	if os.Geteuid() != 0 {
		fatalf("modifying /etc/hosts requires root privileges. Please re-run with sudo.")
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s✖ Error:%s "+format+"\n", append([]any{colorRed, colorReset}, args...)...)
	os.Exit(1)
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
