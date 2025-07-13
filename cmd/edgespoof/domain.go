package main

import (
	"errors"
	"fmt"
	"strings"
)

const (
	maxDomainLength = 253
	maxLabelLength  = 63
)

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
