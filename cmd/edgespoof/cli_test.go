package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want command
	}{
		{"no args shows help", nil, command{name: "help"}},
		{"long help flag", []string{"--help"}, command{name: "help"}},
		{"short help flag", []string{"-h"}, command{name: "help"}},
		{"help flag after command", []string{"set", "-h"}, command{name: "help"}},
		{"version flag", []string{"--version"}, command{name: "version"}},
		{"command ignores case", []string{"LIST"}, command{name: "list"}},
		{"ls alias", []string{"ls"}, command{name: "list"}},
		{"rm alias", []string{"rm", "example.com"}, command{name: "unset", domain: "example.com"}},
		{"resolve", []string{"resolve", "example.com"}, command{name: "resolve", domain: "example.com"}},
		{"domain gets normalized", []string{"resolve", "Example.COM."}, command{name: "resolve", domain: "example.com"}},
		{"flags after domain", []string{"set", "example.com", "-s", "--v4-only"}, command{name: "set", domain: "example.com", staging: true, v4Only: true}},
		{"flags before domain", []string{"set", "--staging", "example.com"}, command{name: "set", domain: "example.com", staging: true}},
		{"double dash ends flags", []string{"resolve", "--", "example.com"}, command{name: "resolve", domain: "example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCommand(tt.args)
			if err != nil {
				t.Fatalf("parseCommand(%q) error: %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("parseCommand(%q) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}

func TestParseCommandErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"unknown command", []string{"pin", "example.com"}, "unknown command"},
		{"missing domain", []string{"set"}, "missing domain"},
		{"two domains", []string{"set", "a.com", "b.com"}, "exactly one domain"},
		{"argument on list", []string{"list", "extra"}, "accepts no arguments"},
		{"argument on version", []string{"version", "extra"}, "accepts no arguments"},
		{"unknown flag", []string{"resolve", "example.com", "--fast"}, "flag provided but not defined"},
		{"staging flag on unset", []string{"unset", "example.com", "-s"}, "flag provided but not defined"},
		{"invalid domain", []string{"set", "bad_name.com"}, "invalid character"},
		{"dash domain after double dash", []string{"resolve", "--", "-x.com"}, "must not start or end"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCommand(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("parseCommand(%q) error = %v, want error containing %q", tt.args, err, tt.wantErr)
			}
		})
	}
}

// Root check must run before any lookup or write
func TestRunRequiresRootForSystemHosts(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root. Test would touch /etc/hosts")
	}
	t.Setenv("EDGESPOOF_HOSTS_PATH", "")
	// No dig. A skipped root check fails with "dig not found" instead
	t.Setenv("PATH", t.TempDir())

	commands := [][]string{
		{"set", "example.com"},
		{"rm", "example.com"},
		{"unset", "example.com"},
		{"clear"},
	}
	for _, args := range commands {
		err := run(args)
		if err == nil || !strings.Contains(err.Error(), "requires root privileges") {
			t.Errorf("run(%q) error = %v, want root privileges error", args, err)
		}
	}
}

func TestRunCustomHostsSkipsRootCheck(t *testing.T) {
	const original = "127.0.0.1 localhost\n"
	path := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDGESPOOF_HOSTS_PATH", path)

	commands := [][]string{
		{"unset", "example.com"},
		{"clear"},
		{"list"},
	}
	for _, args := range commands {
		if err := run(args); err != nil {
			t.Errorf("run(%q) error: %v", args, err)
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != original {
		t.Errorf("hosts file = %q, want %q", content, original)
	}
}
