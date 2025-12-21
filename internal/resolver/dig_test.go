package resolver

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseAnswer(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		recordType string
		wantCNAMEs []string
		wantIP     string
	}{
		{
			name:       "cname chain then first address",
			output:     "www.example.com.edgekey.net.\ne1.a.akamaiedge.net.\n23.50.61.48\n23.50.61.49\n",
			recordType: "A",
			wantCNAMEs: []string{"www.example.com.edgekey.net.", "e1.a.akamaiedge.net."},
			wantIP:     "23.50.61.48",
		},
		{
			name:       "empty output",
			output:     "",
			recordType: "A",
		},
		{
			name:       "blank lines and padding",
			output:     "  \n 1.2.3.4 \n\n",
			recordType: "A",
			wantIP:     "1.2.3.4",
		},
		{
			name:       "A skips IPv6",
			output:     "2606:4700::1\n1.2.3.4\n",
			recordType: "A",
			wantIP:     "1.2.3.4",
		},
		{
			name:       "AAAA skips IPv4",
			output:     "1.2.3.4\n2606:4700::1\n",
			recordType: "AAAA",
			wantIP:     "2606:4700::1",
		},
		{
			name:       "A unmaps IPv4-mapped IPv6",
			output:     "::ffff:1.2.3.4\n",
			recordType: "A",
			wantIP:     "1.2.3.4",
		},
		{
			name:       "AAAA skips IPv4-mapped IPv6",
			output:     "::ffff:1.2.3.4\n",
			recordType: "AAAA",
		},
		{
			name:       "AAAA skips zoned address",
			output:     "fe80::1%en0\n2606:4700::1\n",
			recordType: "AAAA",
			wantIP:     "2606:4700::1",
		},
		{
			name:       "AAAA compacts address",
			output:     "2606:4700:0:0:0:0:0:1\n",
			recordType: "AAAA",
			wantIP:     "2606:4700::1",
		},
		{
			name:       "skips dig comment lines",
			output:     ";; connection timed out; no servers could be reached\n",
			recordType: "A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAnswer(tt.output, tt.recordType)
			if !slices.Equal(got.cnames, tt.wantCNAMEs) {
				t.Errorf("cnames = %q, want %q", got.cnames, tt.wantCNAMEs)
			}
			if got.ip != tt.wantIP {
				t.Errorf("ip = %q, want %q", got.ip, tt.wantIP)
			}
		})
	}
}

// Runs a shell script that fails. Returns its *exec.ExitError
func exitError(t *testing.T, script string) error {
	t.Helper()
	_, err := exec.Command("sh", "-c", script).Output()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("sh -c %q: want exit error, got %v", script, err)
	}
	return err
}

func TestDigError(t *testing.T) {
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	noReply := exitError(t, "exit 9")
	badOption := exitError(t, "echo 'Invalid option: +bogus' >&2; exit 1")
	silent := exitError(t, "exit 1")

	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want string
	}{
		{"deadline", expired, silent, "A lookup for example.com timed out after 5s"},
		{"no reply", context.Background(), noReply, "A lookup for example.com timed out. No DNS server replied"},
		{"stderr detail", context.Background(), badOption, "A lookup for example.com failed: exit status 1 (Invalid option: +bogus)"},
		{"no stderr", context.Background(), silent, "A lookup for example.com failed: exit status 1"},
		{"not an exit error", context.Background(), errors.New("boom"), "A lookup for example.com failed: boom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := digError(tt.ctx, "example.com", "A", tt.err)
			if got.Error() != tt.want {
				t.Errorf("digError = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckDigMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := checkDig()
	if err == nil || !strings.Contains(err.Error(), "dig not found") {
		t.Errorf("checkDig error = %v, want dig not found", err)
	}
}
