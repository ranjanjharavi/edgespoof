package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
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

func printLabeled(color, label, message string) {
	fmt.Fprintf(os.Stderr, "%s%s%s %s\n", color, label, errOut.reset, alignUnderLabel(label, message))
}

// Joined errors span lines. Extra lines start under the first message line
func alignUnderLabel(label, message string) string {
	indent := strings.Repeat(" ", utf8.RuneCountInString(label)+1)
	return strings.ReplaceAll(message, "\n", "\n"+indent)
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

	// MODE stays last. Color codes in an inner column skew tabwriter widths
	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Println()
	fmt.Fprintln(table, "DOMAIN\tIPv4 (A)\tIPv6 (AAAA)\tEDGE TARGET\tUPDATED\tMODE")

	for _, e := range entries {
		mode := "edge"
		modeColor := out.green
		if e.Staging {
			mode = "staging"
			modeColor = out.yellow
		}

		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s%s%s\n", e.Domain, orDash(e.IPv4), orDash(e.IPv6), orDash(e.TargetCNAME), formatUpdated(e.UpdatedAt), modeColor, mode, out.reset)
	}

	table.Flush()
	fmt.Println()
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// Stored as RFC3339 UTC. Shown in local time
func formatUpdated(timestamp string) string {
	updated, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "-"
	}
	return updated.Local().Format(time.DateTime)
}

func printVersion() {
	fmt.Printf("edgespoof %s\n", buildVersion())
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
  edgespoof version                                       Print build version

%sEXAMPLES:%s
  edgespoof resolve example.com
  sudo edgespoof set example.com
  sudo edgespoof set example.com -s
  sudo edgespoof unset example.com
`, out.bold, out.reset, out.cyan, out.reset, out.cyan, out.reset)
}
