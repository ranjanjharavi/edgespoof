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

// NewManager uses /etc/hosts when path is empty
func NewManager(path string) *Manager {
	if path == "" {
		path = DefaultHostsPath
	}

	return &Manager{Path: path}
}

// List returns managed entries in file order
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

func upsertEntry(entries []Entry, incoming Entry) []Entry {
	if incoming.UpdatedAt == "" {
		incoming.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	for i, existing := range entries {
		if strings.EqualFold(existing.Domain, incoming.Domain) {
			entries[i] = incoming
			return entries
		}
	}

	return append(entries, incoming)
}

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

// Remove deletes a domain and reports whether it existed
func (m *Manager) Remove(domain string) (bool, error) {
	return m.update(func(entries []Entry) ([]Entry, bool) {
		return removeEntry(entries, domain)
	})
}

// Clear removes the managed block
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
