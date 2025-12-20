package main

import (
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
