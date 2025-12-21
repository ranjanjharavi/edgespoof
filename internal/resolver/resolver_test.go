package resolver

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFindAndRewriteStaging(t *testing.T) {
	tests := []struct {
		name      string
		cnames    []string
		want      string
		wantFound bool
	}{
		{"edgekey wins over later akamaiedge", []string{"www.example.com.edgekey.net.", "e1.a.akamaiedge.net."}, "www.example.com.edgekey-staging.net.", true},
		{"edgesuite", []string{"www.example.com.edgesuite.net.", "a1.g.akamai.net."}, "www.example.com.edgesuite-staging.net.", true},
		{"akamaiedge", []string{"e1.a.akamaiedge.net."}, "e1.a.akamaiedge-staging.net.", true},
		{"akamai", []string{"a1.g.akamai.net."}, "a1.g.akamai-staging.net.", true},
		{"ignores case", []string{"WWW.Example.COM.EdgeKey.NET."}, "www.example.com.edgekey-staging.net.", true},
		{"other cdn", []string{"www.example.com.cdn.cloudflare.net."}, "", false},
		{"lookalike suffix", []string{"evil-akamai.net."}, "", false},
		{"no cnames", nil, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := findAndRewriteStaging(tt.cnames)
			if got != tt.want || found != tt.wantFound {
				t.Errorf("findAndRewriteStaging(%q) = %q, %v, want %q, %v", tt.cnames, got, found, tt.want, tt.wantFound)
			}
		})
	}
}

func TestStagingTargets(t *testing.T) {
	v4Chain := dnsAnswer{cnames: []string{"v4.example.com.edgekey.net."}}
	v6Chain := dnsAnswer{cnames: []string{"v6.example.com.edgekey.net."}}
	v4Staging := "v4.example.com.edgekey-staging.net."
	v6Staging := "v6.example.com.edgekey-staging.net."

	tests := []struct {
		name      string
		answers   addressResults
		wantIPv4  string
		wantIPv6  string
		wantFound bool
	}{
		{"both families", addressResults{ipv4: v4Chain, ipv6: v6Chain}, v4Staging, v6Staging, true},
		{"IPv4 chain only", addressResults{ipv4: v4Chain}, v4Staging, v4Staging, true},
		{"IPv6 chain only", addressResults{ipv6: v6Chain}, v6Staging, v6Staging, true},
		{"no akamai chain", addressResults{}, "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipv4, ipv6, found := stagingTargets(tt.answers)
			if ipv4 != tt.wantIPv4 || ipv6 != tt.wantIPv6 || found != tt.wantFound {
				t.Errorf("stagingTargets = %q, %q, %v, want %q, %q, %v", ipv4, ipv6, found, tt.wantIPv4, tt.wantIPv6, tt.wantFound)
			}
		})
	}
}

// Prints the canned answer file for the queried name and type.
// A .noreply file exits like an unreachable server
const fakeDigScript = `#!/bin/sh
while [ $# -gt 0 ]; do
	case "$1" in
	-t) type="$2"; shift ;;
	-q) name="$2"; shift ;;
	esac
	shift
done

answer="$FAKE_DIG_ANSWERS/${name}_${type}"
if [ -f "$answer.noreply" ]; then
	exit 9
fi
if [ -f "$answer" ]; then
	cat "$answer"
fi
`

// Puts a fake dig first in PATH. Keys look like "example.com_A"
// Missing keys answer with empty output. noReply keys exit 9
func fakeDig(t *testing.T, answers map[string]string, noReply ...string) {
	t.Helper()
	dir := t.TempDir()
	answerDir := filepath.Join(dir, "answers")
	if err := os.Mkdir(answerDir, 0755); err != nil {
		t.Fatal(err)
	}

	for query, output := range answers {
		writeTestFile(t, filepath.Join(answerDir, query), output, 0644)
	}
	for _, query := range noReply {
		writeTestFile(t, filepath.Join(answerDir, query+".noreply"), "", 0644)
	}
	writeTestFile(t, filepath.Join(dir, "dig"), fakeDigScript, 0755)

	t.Setenv("FAKE_DIG_ANSWERS", answerDir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeTestFile(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
}

// Field by field. One clear message per wrong field
// New Result fields need a line here
func assertResult(t *testing.T, got, want *Result) {
	t.Helper()
	fields := []struct {
		name string
		got  string
		want string
	}{
		{"Domain", got.Domain, want.Domain},
		{"StagingSwap", got.StagingSwap, want.StagingSwap},
		{"TargetCNAME", got.TargetCNAME, want.TargetCNAME},
		{"IPv4", got.IPv4, want.IPv4},
		{"IPv6", got.IPv6, want.IPv6},
	}
	for _, field := range fields {
		if field.got != field.want {
			t.Errorf("%s = %q, want %q", field.name, field.got, field.want)
		}
	}

	if !slices.Equal(got.CNAMEChain, want.CNAMEChain) {
		t.Errorf("CNAMEChain = %q, want %q", got.CNAMEChain, want.CNAMEChain)
	}
	if got.StagingMode != want.StagingMode {
		t.Errorf("StagingMode = %v, want %v", got.StagingMode, want.StagingMode)
	}
	if got.Warning != nil {
		t.Errorf("Warning = %v, want nil", got.Warning)
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name    string
		answers map[string]string
		noReply []string
		domain  string
		staging bool
		v4Only  bool
		want    *Result
	}{
		{
			name: "cname chain sets target edge",
			answers: map[string]string{
				"example.com_A":    "example.com.cdn.example.net.\n1.2.3.4\n",
				"example.com_AAAA": "example.com.cdn.example.net.\n2606:4700::1\n",
			},
			domain: "example.com",
			want: &Result{
				Domain:      "example.com",
				CNAMEChain:  []string{"example.com.cdn.example.net."},
				TargetCNAME: "example.com.cdn.example.net",
				IPv4:        "1.2.3.4",
				IPv6:        "2606:4700::1",
			},
		},
		{
			name:    "no cname uses domain as target",
			answers: map[string]string{"example.com_A": "1.2.3.4\n"},
			domain:  "example.com",
			want: &Result{
				Domain:      "example.com",
				TargetCNAME: "example.com",
				IPv4:        "1.2.3.4",
			},
		},
		{
			name:    "IPv6 only domain",
			answers: map[string]string{"example.com_AAAA": "v6.cdn.example.net.\n2606:4700::1\n"},
			domain:  "example.com",
			want: &Result{
				Domain:      "example.com",
				CNAMEChain:  []string{"v6.cdn.example.net."},
				TargetCNAME: "v6.cdn.example.net",
				IPv6:        "2606:4700::1",
			},
		},
		{
			name:    "v4 only skips AAAA query",
			answers: map[string]string{"example.com_A": "1.2.3.4\n"},
			noReply: []string{"example.com_AAAA"},
			domain:  "example.com",
			v4Only:  true,
			want: &Result{
				Domain:      "example.com",
				TargetCNAME: "example.com",
				IPv4:        "1.2.3.4",
			},
		},
		{
			name: "staging swaps edge hostname",
			answers: map[string]string{
				"www.example.com_A":                         "www.example.com.edgekey.net.\ne1.a.akamaiedge.net.\n23.1.1.1\n",
				"www.example.com_AAAA":                      "www.example.com.edgekey.net.\ne1.a.akamaiedge.net.\n2600::1\n",
				"www.example.com.edgekey-staging.net._A":    "e1.a.akamaiedge-staging.net.\n23.9.9.9\n",
				"www.example.com.edgekey-staging.net._AAAA": "e1.a.akamaiedge-staging.net.\n2600::9\n",
			},
			domain:  "www.example.com",
			staging: true,
			want: &Result{
				Domain:      "www.example.com",
				CNAMEChain:  []string{"www.example.com.edgekey.net.", "e1.a.akamaiedge.net."},
				StagingSwap: "www.example.com.edgekey-staging.net.",
				TargetCNAME: "e1.a.akamaiedge-staging.net",
				IPv4:        "23.9.9.9",
				IPv6:        "2600::9",
				StagingMode: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeDig(t, tt.answers, tt.noReply...)

			got, err := Resolve(context.Background(), tt.domain, tt.staging, tt.v4Only)
			if err != nil {
				t.Fatalf("Resolve error: %v", err)
			}
			assertResult(t, got, tt.want)
		})
	}
}

func TestResolvePartialFailureWarns(t *testing.T) {
	fakeDig(t, map[string]string{"example.com_A": "1.2.3.4\n"}, "example.com_AAAA")

	got, err := Resolve(context.Background(), "example.com", false, false)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if got.IPv4 != "1.2.3.4" {
		t.Errorf("IPv4 = %q, want 1.2.3.4", got.IPv4)
	}
	if got.Warning == nil || !strings.Contains(got.Warning.Error(), "AAAA lookup for example.com timed out") {
		t.Errorf("Warning = %v, want AAAA timeout", got.Warning)
	}
}

func TestResolveErrors(t *testing.T) {
	tests := []struct {
		name     string
		answers  map[string]string
		noReply  []string
		staging  bool
		wantErrs []string
	}{
		{
			name:     "no records",
			wantErrs: []string{"no valid A or AAAA records found for example.com"},
		},
		{
			name:     "both families fail",
			noReply:  []string{"example.com_A", "example.com_AAAA"},
			wantErrs: []string{"A lookup for example.com timed out", "AAAA lookup for example.com timed out"},
		},
		{
			name:     "staging without akamai cname",
			answers:  map[string]string{"example.com_A": "1.2.3.4\n"},
			staging:  true,
			wantErrs: []string{"no supported Akamai CNAME found for example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeDig(t, tt.answers, tt.noReply...)

			_, err := Resolve(context.Background(), "example.com", tt.staging, false)
			if err == nil {
				t.Fatal("Resolve error = nil, want error")
			}
			for _, want := range tt.wantErrs {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Resolve error = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}
