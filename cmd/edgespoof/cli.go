package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yupcode/edgespoof/internal/hosts"
	"github.com/yupcode/edgespoof/internal/resolver"
)

const (
	maxDomainLength = 253
	maxLabelLength  = 63
)

type command struct {
	name    string
	domain  string
	staging bool
	v4Only  bool
}

func parseCommand(args []string) (command, error) {
	if len(args) == 0 {
		return command{name: "help"}, nil
	}

	cmd := command{name: strings.ToLower(args[0])}
	switch cmd.name {
	case "rm":
		cmd.name = "unset"
	case "ls":
		cmd.name = "list"
	case "-h", "--help":
		cmd.name = "help"
	case "resolve", "set", "unset", "list", "status", "clear", "help":
	default:
		return command{}, fmt.Errorf("unknown command %q. Run 'edgespoof help' for usage", args[0])
	}

	fs := flag.NewFlagSet(cmd.name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if cmd.name == "resolve" || cmd.name == "set" {
		fs.BoolVar(&cmd.staging, "staging", false, "Use the staging network")
		fs.BoolVar(&cmd.staging, "s", false, "Use the staging network")
		fs.BoolVar(&cmd.v4Only, "v4-only", false, "Resolve IPv4 only")
	}

	flagArgs, positional := splitArguments(args[1:])
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return command{name: "help"}, nil
		}

		return command{}, fmt.Errorf("%s: %w", cmd.name, err)
	}

	switch cmd.name {
	case "resolve", "set", "unset":
		if len(positional) == 0 {
			return command{}, fmt.Errorf("missing domain argument. Usage: edgespoof %s <domain>", cmd.name)
		}
		if len(positional) > 1 {
			return command{}, fmt.Errorf("%s accepts exactly one domain, unexpected argument %q", cmd.name, positional[1])
		}

		domain, err := normalizeDomain(positional[0])
		if err != nil {
			return command{}, err
		}
		cmd.domain = domain
	default:
		if len(positional) > 0 {
			return command{}, fmt.Errorf("%s accepts no arguments, unexpected argument %q", cmd.name, positional[0])
		}
	}

	return cmd, nil
}

func splitArguments(args []string) (flags, positional []string) {
	for i, arg := range args {
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flags = append(flags, arg)
			continue
		}
		positional = append(positional, arg)
	}

	return flags, positional
}

// Hostname rules: letters, digits, and '-' per label. Labels join with '.'
func normalizeDomain(domain string) (string, error) {
	name := strings.TrimSuffix(domain, ".")
	if name == "" {
		return "", errors.New("domain must not be empty")
	}
	if len(name) > maxDomainLength {
		return "", fmt.Errorf("domain %q is longer than %d characters", domain, maxDomainLength)
	}

	for _, label := range strings.Split(name, ".") {
		if err := validateLabel(label); err != nil {
			return "", fmt.Errorf("invalid domain %q: %w", domain, err)
		}
	}

	return strings.ToLower(name), nil
}

func validateLabel(label string) error {
	if label == "" {
		return errors.New("empty label")
	}
	if len(label) > maxLabelLength {
		return fmt.Errorf("label %q is longer than %d characters", label, maxLabelLength)
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return fmt.Errorf("label %q must not start or end with '-'", label)
	}

	for _, char := range label {
		if !isLabelChar(char) {
			return fmt.Errorf("label %q has invalid character %q", label, char)
		}
	}
	return nil
}

func isLabelChar(char rune) bool {
	if char == '-' {
		return true
	}
	if 'a' <= char && char <= 'z' {
		return true
	}
	if 'A' <= char && char <= 'Z' {
		return true
	}
	return '0' <= char && char <= '9'
}

func run(args []string) error {
	cmd, err := parseCommand(args)
	if err != nil {
		return err
	}

	customHostsPath := os.Getenv("EDGESPOOF_HOSTS_PATH")
	customHosts := customHostsPath != ""
	mgr := hosts.NewManager(customHostsPath)

	switch cmd.name {
	case "set", "unset", "clear":
		if !customHosts && os.Geteuid() != 0 {
			return errors.New("modifying /etc/hosts requires root privileges. Re-run with sudo")
		}
	}

	switch cmd.name {
	case "resolve", "set":
		return runResolve(cmd, mgr, customHosts)

	case "unset":
		removed, err := mgr.Remove(cmd.domain)
		if err != nil {
			return fmt.Errorf("failed to update hosts file: %w", err)
		}
		if !removed {
			printDomainNotFound(cmd.domain)
			return nil
		}
		message := fmt.Sprintf("Removed %s from hosts file", cmd.domain)
		reportHostsUpdate(message, customHosts)

	case "list":
		entries, err := mgr.List()
		if err != nil {
			return fmt.Errorf("failed to read hosts file: %w", err)
		}
		printEntries(entries)

	case "status":
		entries, err := mgr.List()
		if err != nil || len(entries) == 0 {
			return nil
		}
		printStatus(len(entries))

	case "clear":
		if err := mgr.Clear(); err != nil {
			return fmt.Errorf("failed to clear hosts block: %w", err)
		}
		reportHostsUpdate("Cleared all edgespoof entries", customHosts)

	case "help":
		printUsage()
	}

	return nil
}

func runResolve(cmd command, mgr *hosts.Manager, customHosts bool) error {
	res, err := resolver.Resolve(context.Background(), cmd.domain, cmd.staging, cmd.v4Only)
	if err != nil {
		return err
	}

	printResolution(res)
	if res.Warning != nil {
		printLookupWarning(res.Warning)
	}
	if cmd.name == "resolve" {
		return nil
	}

	err = mgr.Upsert(hosts.Entry{
		Domain:      res.Domain,
		IPv4:        res.IPv4,
		IPv6:        res.IPv6,
		TargetCNAME: res.TargetCNAME,
		Staging:     res.StagingMode,
	})
	if err != nil {
		return fmt.Errorf("failed to update hosts file: %w", err)
	}

	reportHostsUpdate("Updated hosts file", customHosts)
	return nil
}
