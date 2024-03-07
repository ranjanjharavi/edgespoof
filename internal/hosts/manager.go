package hosts

import (
	"os"
	"strings"
)

const (
	DefaultHostsPath = "/etc/hosts"
	FenceStart       = "# --- EDGESPOOF START ---"
	FenceEnd         = "# --- EDGESPOOF END ---"
)

type Manager struct {
	Path string
}

// Returns a Manager pointing to path (or /etc/hosts if empty)
func NewManager(path string) *Manager {
	if path == "" {
		path = DefaultHostsPath
	}

	return &Manager{Path: path}
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
