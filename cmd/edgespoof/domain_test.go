package main

import (
	"strings"
	"testing"
)

func TestNormalizeDomain(t *testing.T) {
	longestLabel := strings.Repeat("a", maxLabelLength) + ".com"
	longestName := strings.Repeat("a.", 126) + "a"

	tests := []struct {
		input string
		want  string
	}{
		{"example.com", "example.com"},
		{"Example.COM", "example.com"},
		{"example.com.", "example.com"},
		{"localhost", "localhost"},
		{"a-b.c-d.example", "a-b.c-d.example"},
		{"123.example.com", "123.example.com"},
		{"xn--bcher-kva.de", "xn--bcher-kva.de"},
		{longestLabel, longestLabel},
		{longestName, longestName},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizeDomain(tt.input)
			if err != nil {
				t.Fatalf("normalizeDomain(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("normalizeDomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeDomainErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"empty", "", "must not be empty"},
		{"only dot", ".", "must not be empty"},
		{"double trailing dot", "example.com..", "empty label"},
		{"leading dot", ".example.com", "empty label"},
		{"inner empty label", "example..com", "empty label"},
		{"dig server syntax", "@1.1.1.1", "invalid character"},
		{"dig option syntax", "+short", "invalid character"},
		{"hosts comment char", "foo#bar.com", "invalid character"},
		{"slash", "a/b.com", "invalid character"},
		{"underscore", "under_score.com", "invalid character"},
		{"space", "exa mple.com", "invalid character"},
		{"control char", "example\x00.com", "invalid character"},
		{"unicode", "bücher.de", "invalid character"},
		{"kelvin sign lowercases to ascii k", "Kexample.com", "invalid character"},
		{"leading hyphen", "-example.com", "must not start or end"},
		{"trailing hyphen", "example-.com", "must not start or end"},
		{"label too long", strings.Repeat("a", maxLabelLength+1) + ".com", "longer than 63"},
		{"name too long", strings.Repeat("a.", 127) + "a", "longer than 253"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeDomain(tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("normalizeDomain(%q) error = %v, want error containing %q", tt.input, err, tt.wantErr)
			}
		})
	}
}
