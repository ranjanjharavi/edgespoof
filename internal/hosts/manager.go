package hosts

import (
	"os"
	"strings"
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

// Upsert adds or updates a domain inside the managed block
func (m *Manager) Upsert(e Entry) error {
	_, err := m.update(func(entries []Entry) ([]Entry, bool) {
		return upsertEntry(entries, e), true
	})

	return err
}

// Remove deletes a domain from the managed block.
func (m *Manager) Remove(domain string) (bool, error) {
	return m.update(func(entries []Entry) ([]Entry, bool) {
		return removeEntry(entries, domain)
	})
}

// Clear removes the managed block.
func (m *Manager) Clear() error {
	_, err := m.update(func([]Entry) ([]Entry, bool) {
		return nil, true
	})

	return err
}

func (m *Manager) update(change func([]Entry) ([]Entry, bool)) (bool, error) {
	lock, err := m.lockFile()
	if err != nil {
		return false, err
	}
	defer lock.Close()

	content, err := os.ReadFile(m.Path)
	if err != nil {
		return false, err
	}
	sections, err := parseFileContent(string(content))
	if err != nil {
		return false, err
	}
	entries := parseEntries(contentLines(sections.block))
	entries, changed := change(entries)
	if !changed {
		return false, nil
	}

	updatedContent := formatFileContent(sections, entries)
	if updatedContent == string(content) {
		return true, nil
	}

	return true, m.writeAtomic(updatedContent)
}
