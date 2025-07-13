package main

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/yupcode/edgespoof/internal/hosts"
	"github.com/yupcode/edgespoof/internal/resolver"
)

type palette struct {
	reset  string
	cyan   string
	green  string
	yellow string
	red    string
	dim    string
	bold   string
}

var ansi = palette{
	reset:  "\033[0m",
	cyan:   "\033[36m",
	green:  "\033[32m",
	yellow: "\033[33m",
	red:    "\033[31m",
	dim:    "\033[2m",
	bold:   "\033[1m",
}

// One palette per stream. Each stream can be a terminal or a pipe
var (
	out    = paletteFor(os.Stdout)
	errOut = paletteFor(os.Stderr)
)

// Any non-empty NO_COLOR disables colors
func paletteFor(file *os.File) palette {
	if os.Getenv("NO_COLOR") != "" {
		return palette{}
	}

	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return palette{}
	}
	return ansi
}

// Joined errors span lines. Extra lines start under the first message line
func printLabeled(color, label, message string) {
	indent := strings.Repeat(" ", utf8.RuneCountInString(label)+1)
	message = strings.ReplaceAll(message, "\n", "\n"+indent)
	fmt.Fprintf(os.Stderr, "%s%s%s %s\n", color, label, errOut.reset, message)
}

func printError(err error) {
	printLabeled(errOut.red, "✖ Error:", err.Error())
}

func printSuccess(message string) {
	fmt.Printf("%s✔%s %s\n", out.green, out.reset, message)
}

func printFlushWarning(err error) {
	printLabeled(errOut.yellow, "! Warning:", "Hosts file updated, but OS DNS cache flush failed: "+err.Error())
}

func printLookupWarning(err error) {
	printLabeled(errOut.yellow, "! Warning:", "Partial DNS lookup: "+err.Error())
}

func printDomainNotFound(domain string) {
	fmt.Printf("%s!%s Domain %s was not found in the active edgespoof block.\n", out.yellow, out.reset, domain)
}

func printStatus(count int) {
	fmt.Printf("[SPOOF:%d]\n", count)
}

func printResolution(res *resolver.Result) {
	fmt.Printf("\n%sDomain%s       : %s%s%s\n", out.cyan, out.reset, out.bold, res.Domain, out.reset)

	for _, cname := range res.CNAMEChain {
		fmt.Printf("%sCNAME%s        : -> %s\n", out.cyan, out.reset, cname)
	}

	if res.StagingSwap != "" {
		fmt.Printf("%sStaging Swap%s : -> %s%s%s\n", out.yellow, out.reset, out.yellow, res.StagingSwap, out.reset)
	}

	fmt.Printf("%sTarget Edge%s  : %s%s%s\n", out.cyan, out.reset, out.green, res.TargetCNAME, out.reset)

	if res.IPv4 != "" {
		fmt.Printf("%sIPv4 (A)%s     : %s%s%s\n", out.cyan, out.reset, out.bold, res.IPv4, out.reset)
	} else {
		fmt.Printf("%sIPv4 (A)%s     : %snone%s\n", out.cyan, out.reset, out.dim, out.reset)
	}

	if res.IPv6 != "" {
		fmt.Printf("%sIPv6 (AAAA)%s  : %s%s%s\n", out.cyan, out.reset, out.bold, res.IPv6, out.reset)
	} else {
		fmt.Printf("%sIPv6 (AAAA)%s  : %snone%s\n", out.cyan, out.reset, out.dim, out.reset)
	}

	fmt.Println()
}

func printEntries(entries []hosts.Entry) {
	if len(entries) == 0 {
		fmt.Printf("%sNo active edgespoof entries in hosts file.%s\n", out.dim, out.reset)
		return
	}

	fmt.Printf("\n%s%-28s %-16s %-30s %-10s %-34s%s\n", out.bold, "DOMAIN", "IPv4 (A)", "IPv6 (AAAA)", "MODE", "EDGE TARGET", out.reset)
	fmt.Printf("%s%s%s\n", out.dim, strings.Repeat("-", 122), out.reset)

	for _, e := range entries {
		v4 := e.IPv4
		if v4 == "" {
			v4 = "-"
		}

		v6 := e.IPv6
		if v6 == "" {
			v6 = "-"
		}

		mode := "edge"
		modeColor := out.green
		if e.Staging {
			mode = "staging"
			modeColor = out.yellow
		}

		fmt.Printf("%-28s %-16s %-30s %s%-10s%s %-34s\n", e.Domain, v4, v6, modeColor, mode, out.reset, e.TargetCNAME)
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
`, out.bold, out.reset, out.cyan, out.reset, out.cyan, out.reset)
}
