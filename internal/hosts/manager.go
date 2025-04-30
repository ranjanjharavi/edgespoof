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

const DefaultHostsPath = "/etc/hosts"

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

	sections, err := parseFileContent(string(content))
	if err != nil {
		return nil, err
	}
	lines := contentLines(sections.block)
	entries := parseEntries(lines)
	return entries, nil
}

// Reads the hosts file and splits it into before, block, and after sections
func (m *Manager) ReadSections() (before, block, after []string, err error) {
	content, err := os.ReadFile(m.Path)
	if err != nil {
		return nil, nil, nil, err
	}

	sections, err := parseFileContent(string(content))
	if err != nil {
		return nil, nil, nil, err
	}
	before = contentLines(sections.before)
	block = contentLines(sections.block)
	after = contentLines(sections.after)
	return before, block, after, nil
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

// Upsert a domain's IPv4 and IPv6 records inside the fenced block atomically
func (m *Manager) Upsert(e Entry) error {
	content, err := os.ReadFile(m.Path)
	if err != nil {
		return err
	}

	sections, err := parseFileContent(string(content))
	if err != nil {
		return err
	}
	lines := contentLines(sections.block)
	entries := parseEntries(lines)
	entries = upsertEntry(entries, e)

	updatedContent := formatFileContent(sections, entries)
	return m.writeAtomic(updatedContent)
}

// Deletes a specific domain from the fenced block atomically
func (m *Manager) Remove(domain string) (bool, error) {
	content, err := os.ReadFile(m.Path)
	if err != nil {
		return false, err
	}

	sections, err := parseFileContent(string(content))
	if err != nil {
		return false, err
	}
	lines := contentLines(sections.block)
	entries := parseEntries(lines)
	entries, removed := removeEntry(entries, domain)
	if !removed {
		return false, nil
	}

	updatedContent := formatFileContent(sections, entries)
	return true, m.writeAtomic(updatedContent)
}

// Clear removes the entire fenced block from the hosts file atomically
func (m *Manager) Clear() error {
	content, err := os.ReadFile(m.Path)
	if err != nil {
		return err
	}

	sections, err := parseFileContent(string(content))
	if err != nil {
		return err
	}
	if !sections.hasBlock {
		return nil
	}
	updatedContent := formatFileContent(sections, nil)
	return m.writeAtomic(updatedContent)
}

// Writes the formatted hosts content to a sibling temp file,
// sets 0644 permissions, and atomically swaps it over the target path
func (m *Manager) writeAtomic(finalData string) error {
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
