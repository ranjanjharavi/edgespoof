package hosts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultHostsPath = "/etc/hosts"
	FenceStart       = "# --- EDGESPOOF START ---"
	FenceEnd         = "# --- EDGESPOOF END ---"
)

type Manager struct {
	Path string
}

type Entry struct {
	Domain      string
	IPv4        string
	IPv6        string
	TargetCNAME string
	Staging     bool
	UpdatedAt   string
}

// Returns a Manager pointing to path (or /etc/hosts if empty)
func NewManager(path string) *Manager {
	if path == "" {
		path = DefaultHostsPath
	}

	return &Manager{Path: path}
}

// Reads the hosts file and returns all active spoofed domains in order
func (m *Manager) List() ([]Entry, error) {
	content, err := os.ReadFile(m.Path)
	if err != nil {
		return nil, err
	}

	_, blockLines, _ := splitContent(string(content))
	return parseEntries(blockLines), nil
}

// Reads the hosts file and splits it into before, block, and after sections
func (m *Manager) ReadSections() (before, block, after []string, err error) {
	content, err := os.ReadFile(m.Path)
	if err != nil {
		return nil, nil, nil, err
	}

	before, block, after = splitContent(string(content))
	return before, block, after, nil
}

// Isolate the EDGESPOOF block
func splitContent(raw string) (before, block, after []string) {
	// Normalize Windows (\r\n) line endings to Unix (\n) just in case
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")

	// 0 = before fence, 1 = inside fence, 2 = after fence
	state := 0

	// Transition on the markers but do not include them
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == FenceStart {
			state = 1
			continue
		}

		if trimmed == FenceEnd {
			state = 2
			continue
		}

		switch state {
		case 0:
			before = append(before, line)
		case 1:
			block = append(block, line)
		case 2:
			after = append(after, line)
		}
	}

	return before, block, after
}

// Merges IPv4/IPv6 lines and extracts inline comment metadata
func parseEntries(lines []string) []Entry {
	byDomain := make(map[string]*Entry)
	var order []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Split the line into the hosts part and the inline comment
		parts := strings.SplitN(trimmed, "#", 2)

		// Extract IP and Domain using strings.Fields (handles any number of spaces/tabs)
		hostFields := strings.Fields(parts[0])
		if len(hostFields) < 2 {
			continue
		}

		ip := hostFields[0]
		domain := strings.ToLower(hostFields[1])

		// Look up or initialize the Entry for this domain while preserving insertion order
		ent, exists := byDomain[domain]
		if !exists {
			ent = &Entry{Domain: domain}
			byDomain[domain] = ent
			order = append(order, domain)
		}

		// Assign IP to IPv6 (contains ':') or IPv4
		if strings.Contains(ip, ":") {
			ent.IPv6 = ip
		} else {
			ent.IPv4 = ip
		}

		// Parse inline metadata if a comment exists
		if len(parts) == 2 {
			parseCommentMetadata(parts[1], ent)
		}
	}

	// Reconstruct the ordered slice of Entries
	var result []Entry
	for _, domain := range order {
		result = append(result, *byDomain[domain])
	}
	return result
}

// parseCommentMetadata extracts key=value pairs from "edgespoof:target=...|mode=...|ts=..."
func parseCommentMetadata(rawComment string, ent *Entry) {
	meta := strings.TrimSpace(rawComment)
	if !strings.HasPrefix(meta, "edgespoof:") {
		return
	}
	meta = strings.TrimPrefix(meta, "edgespoof:")

	for _, kv := range strings.Split(meta, "|") {
		pair := strings.SplitN(kv, "=", 2)
		if len(pair) != 2 {
			continue
		}
		key, val := pair[0], pair[1]

		switch key {
		case "target":
			ent.TargetCNAME = val
		case "mode":
			ent.Staging = (val == "staging")
		case "ts":
			ent.UpdatedAt = val
		}
	}
}

// Update an existing domain or append if new
func upsertEntry(entries []Entry, incoming Entry) []Entry {
	if incoming.UpdatedAt == "" {
		incoming.UpdatedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}

	for i, existing := range entries {
		if strings.EqualFold(existing.Domain, incoming.Domain) {
			entries[i] = incoming // Update in-place to preserve ordering
			return entries
		}
	}

	return append(entries, incoming)
}

// Filter a domain and report if found
func removeEntry(entries []Entry, domain string) ([]Entry, bool) {
	var filtered []Entry
	removed := false

	for _, existing := range entries {
		if strings.EqualFold(existing.Domain, domain) {
			removed = true
			continue
		}
		filtered = append(filtered, existing)
	}

	return filtered, removed
}

// Reconstruct the entire hosts file with no blank line drifts
func formatFileContent(before []string, entries []Entry, after []string) string {
	var out []string

	// Trim from 'before'
	for len(before) > 0 && strings.TrimSpace(before[len(before)-1]) == "" {
		before = before[:len(before)-1]
	}
	out = append(out, before...)

	// Render edgespoof block ONLY
	if len(entries) > 0 {
		if len(out) > 0 {
			out = append(out, "") // Single blank line before fence
		}
		out = append(out, FenceStart)

		for _, e := range entries {
			mode := "edge"
			if e.Staging {
				mode = "staging"
			}
			meta := fmt.Sprintf("# edgespoof:target=%s|mode=%s|ts=%s", e.TargetCNAME, mode, e.UpdatedAt)

			if e.IPv4 != "" {
				out = append(out, fmt.Sprintf("%-28s %-32s %s", e.IPv4, e.Domain, meta))
			}
			if e.IPv6 != "" {
				out = append(out, fmt.Sprintf("%-28s %-32s %s", e.IPv6, e.Domain, meta))
			}
		}

		out = append(out, FenceEnd)
	}

	// Trim from 'after'
	for len(after) > 0 && strings.TrimSpace(after[0]) == "" {
		after = after[1:]
	}

	// Trim trailing blank lines from 'after' so we end with a single clean "\n"
	for len(after) > 0 && strings.TrimSpace(after[len(after)-1]) == "" {
		after = after[:len(after)-1]
	}

	if len(after) > 0 {
		if len(out) > 0 {
			out = append(out, "") // Single blank line after fence
		}
		out = append(out, after...)
	}

	return strings.Join(out, "\n") + "\n"
}

// Upsert a domain's IPv4 and IPv6 records inside the fenced block atomically
func (m *Manager) Upsert(e Entry) error {
	content, err := os.ReadFile(m.Path)
	if err != nil {
		return err
	}

	before, blockLines, after := splitContent(string(content))
	entries := upsertEntry(parseEntries(blockLines), e)

	return m.writeAtomic(before, entries, after)
}

// Deletes a specific domain from the fenced block atomically
func (m *Manager) Remove(domain string) (bool, error) {
	content, err := os.ReadFile(m.Path)
	if err != nil {
		return false, err
	}

	before, blockLines, after := splitContent(string(content))
	entries, removed := removeEntry(parseEntries(blockLines), domain)
	if !removed {
		return false, nil
	}

	return true, m.writeAtomic(before, entries, after)
}

// Clear removes the entire fenced block from the hosts file atomically
func (m *Manager) Clear() error {
	content, err := os.ReadFile(m.Path)
	if err != nil {
		return err
	}

	before, _, after := splitContent(string(content))
	return m.writeAtomic(before, nil, after)
}

// Writes the formatted hosts content to a sibling temp file,
// sets 0644 permissions, and atomically swaps it over the target path
func (m *Manager) writeAtomic(before []string, entries []Entry, after []string) error {
	finalData := formatFileContent(before, entries, after)

	// Create temp file in the exact same directory to guarantee same-filesystem rename
	dir := filepath.Dir(m.Path)
	tmpFile, err := os.CreateTemp(dir, ".hosts.edgespoof.*")
	if err != nil {
		return fmt.Errorf("failed to create temp hosts file (verify sudo?): %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName) // Ensure cleanup if any step before Rename fails

	// Write formatted content to the temp file
	if _, err := tmpFile.WriteString(finalData); err != nil {
		tmpFile.Close()
		return err
	}

	// Critical: os.CreateTemp defaults to 0600.
	// Restore 0644 so non-root processes can read /etc/hosts
	if err := tmpFile.Chmod(0644); err != nil {
		tmpFile.Close()
		return err
	}

	// Flush and close descriptor before renaming
	if err := tmpFile.Close(); err != nil {
		return err
	}

	// Atomic POSIX rename over the target file
	if err := os.Rename(tmpName, m.Path); err != nil {
		// Fallback for Docker / bind-mounted /etc/hosts where rename returns EXDEV or EBUSY
		if errors.Is(err, syscall.EXDEV) || errors.Is(err, syscall.EBUSY) {
			return os.WriteFile(m.Path, []byte(finalData), 0644)
		}
		return err
	}

	return nil
}
