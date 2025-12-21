package hosts

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

const originalHosts = "127.0.0.1 localhost\n::1 localhost\n"

func newTestManager(t *testing.T, content string, perm os.FileMode) *Manager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
	return NewManager(path)
}

func readHosts(t *testing.T, mgr *Manager) string {
	t.Helper()
	content, err := os.ReadFile(mgr.Path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func mustUpsert(t *testing.T, mgr *Manager, entry Entry) {
	t.Helper()
	if err := mgr.Upsert(entry); err != nil {
		t.Fatalf("Upsert(%s) error: %v", entry.Domain, err)
	}
}

func assertEntries(t *testing.T, mgr *Manager, want []Entry) {
	t.Helper()
	got, err := mgr.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("List\n got %+v\nwant %+v", got, want)
	}
}

func TestNewManagerDefaultPath(t *testing.T) {
	if got := NewManager("").Path; got != DefaultHostsPath {
		t.Errorf("NewManager(\"\").Path = %q, want %q", got, DefaultHostsPath)
	}
}

func TestManagerUpsertSetsTimestamp(t *testing.T) {
	mgr := newTestManager(t, originalHosts, 0644)
	mustUpsert(t, mgr, Entry{Domain: "example.com", IPv4: "1.2.3.4"})

	entries, err := mgr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("List returned %d entries, want 1", len(entries))
	}
	if _, err := time.Parse(time.RFC3339, entries[0].UpdatedAt); err != nil {
		t.Errorf("UpdatedAt %q is not RFC3339: %v", entries[0].UpdatedAt, err)
	}
}

func TestManagerUpsertKeepsUnmanagedContent(t *testing.T) {
	mgr := newTestManager(t, originalHosts, 0644)
	mustUpsert(t, mgr, Entry{Domain: "example.com", IPv4: "1.2.3.4", UpdatedAt: "2026-11-07T05:00:00Z"})

	content := readHosts(t, mgr)
	sections, err := parseFileContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if sections.before != originalHosts {
		t.Errorf("content before block = %q, want %q", sections.before, originalHosts)
	}
}

func TestManagerUpsertReplacesInPlace(t *testing.T) {
	mgr := newTestManager(t, originalHosts, 0644)
	first := Entry{Domain: "a.com", IPv4: "1.1.1.1", UpdatedAt: "2026-11-07T05:00:00Z"}
	second := Entry{Domain: "b.com", IPv4: "2.2.2.2", UpdatedAt: "2026-11-07T05:00:00Z"}
	replaced := Entry{Domain: "a.com", IPv4: "3.3.3.3", Staging: true, UpdatedAt: "2026-11-07T06:00:00Z"}

	mustUpsert(t, mgr, first)
	mustUpsert(t, mgr, second)
	mustUpsert(t, mgr, replaced)

	assertEntries(t, mgr, []Entry{replaced, second})
}

func TestManagerRemove(t *testing.T) {
	mgr := newTestManager(t, originalHosts, 0644)
	first := Entry{Domain: "a.com", IPv4: "1.1.1.1", UpdatedAt: "2026-11-07T05:00:00Z"}
	second := Entry{Domain: "b.com", IPv4: "2.2.2.2", UpdatedAt: "2026-11-07T05:00:00Z"}
	mustUpsert(t, mgr, first)
	mustUpsert(t, mgr, second)

	removed, err := mgr.Remove("a.com")
	if err != nil || !removed {
		t.Fatalf("Remove(a.com) = %v, %v, want true, nil", removed, err)
	}
	assertEntries(t, mgr, []Entry{second})

	before := readHosts(t, mgr)
	removed, err = mgr.Remove("a.com")
	if err != nil || removed {
		t.Fatalf("second Remove(a.com) = %v, %v, want false, nil", removed, err)
	}
	if after := readHosts(t, mgr); after != before {
		t.Errorf("Remove of missing domain changed file\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestManagerRemoveLastEntryRestoresFile(t *testing.T) {
	mgr := newTestManager(t, originalHosts, 0644)
	mustUpsert(t, mgr, Entry{Domain: "example.com", IPv4: "1.2.3.4"})

	if _, err := mgr.Remove("example.com"); err != nil {
		t.Fatal(err)
	}
	if got := readHosts(t, mgr); got != originalHosts {
		t.Errorf("hosts file = %q, want %q", got, originalHosts)
	}
}

func TestManagerClear(t *testing.T) {
	mgr := newTestManager(t, originalHosts, 0644)
	mustUpsert(t, mgr, Entry{Domain: "a.com", IPv4: "1.1.1.1"})
	mustUpsert(t, mgr, Entry{Domain: "b.com", IPv6: "2606:4700::1"})

	if err := mgr.Clear(); err != nil {
		t.Fatal(err)
	}
	if got := readHosts(t, mgr); got != originalHosts {
		t.Errorf("hosts file = %q, want %q", got, originalHosts)
	}

	// Nothing left to clear. Still no error
	if err := mgr.Clear(); err != nil {
		t.Errorf("second Clear error: %v", err)
	}
}

func TestManagerKeepsFileMode(t *testing.T) {
	mgr := newTestManager(t, originalHosts, 0600)
	mustUpsert(t, mgr, Entry{Domain: "example.com", IPv4: "1.2.3.4"})

	info, err := os.Stat(mgr.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("file mode = %o, want 600", got)
	}
}

func TestManagerBrokenBlockLeavesFileAlone(t *testing.T) {
	broken := originalHosts + FenceStart + "\n1.2.3.4 example.com\n"
	mgr := newTestManager(t, broken, 0644)

	if _, err := mgr.List(); err == nil {
		t.Error("List error = nil, want missing end marker error")
	}
	if err := mgr.Upsert(Entry{Domain: "other.com", IPv4: "5.6.7.8"}); err == nil {
		t.Error("Upsert error = nil, want missing end marker error")
	}
	if got := readHosts(t, mgr); got != broken {
		t.Errorf("hosts file changed\n got %q\nwant %q", got, broken)
	}
}

func TestManagerMissingFile(t *testing.T) {
	mgr := NewManager(filepath.Join(t.TempDir(), "missing"))

	if _, err := mgr.List(); err == nil {
		t.Error("List error = nil, want not found error")
	}
	if err := mgr.Upsert(Entry{Domain: "example.com", IPv4: "1.2.3.4"}); err == nil {
		t.Error("Upsert error = nil, want not found error")
	}
}
