package hosts

import (
	"slices"
	"strings"
	"testing"
)

func TestParseFileContent(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want fileSections
	}{
		{
			name: "empty file",
			raw:  "",
			want: fileSections{newline: "\n"},
		},
		{
			name: "no block",
			raw:  "127.0.0.1 localhost\n",
			want: fileSections{before: "127.0.0.1 localhost\n", newline: "\n"},
		},
		{
			name: "block between content",
			raw:  "127.0.0.1 localhost\n" + FenceStart + "\n1.2.3.4 example.com\n" + FenceEnd + "\n# after\n",
			want: fileSections{
				before:   "127.0.0.1 localhost\n",
				block:    "1.2.3.4 example.com\n",
				after:    "# after\n",
				newline:  "\n",
				hasBlock: true,
			},
		},
		{
			name: "crlf line endings",
			raw:  "127.0.0.1 localhost\r\n" + FenceStart + "\r\n1.2.3.4 example.com\r\n" + FenceEnd + "\r\n",
			want: fileSections{
				before:   "127.0.0.1 localhost\r\n",
				block:    "1.2.3.4 example.com\r\n",
				newline:  "\r\n",
				hasBlock: true,
			},
		},
		{
			name: "markers with surrounding whitespace",
			raw:  "  " + FenceStart + "  \n1.2.3.4 example.com\n\t" + FenceEnd + "\n",
			want: fileSections{
				block:    "1.2.3.4 example.com\n",
				newline:  "\n",
				hasBlock: true,
			},
		},
		{
			name: "end marker without final newline",
			raw:  FenceStart + "\n1.2.3.4 example.com\n" + FenceEnd,
			want: fileSections{
				block:    "1.2.3.4 example.com\n",
				newline:  "\n",
				hasBlock: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFileContent(tt.raw)
			if err != nil {
				t.Fatalf("parseFileContent error: %v", err)
			}
			if got != tt.want {
				t.Errorf("parseFileContent(%q)\n got %+v\nwant %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseFileContentErrors(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{"duplicate start", FenceStart + "\n" + FenceStart + "\n" + FenceEnd + "\n", "duplicate start marker"},
		{"end before start", FenceEnd + "\n" + FenceStart + "\n", "end marker before start marker"},
		{"duplicate end", FenceStart + "\n" + FenceEnd + "\n" + FenceEnd + "\n", "duplicate end marker"},
		{"missing end", "127.0.0.1 localhost\n" + FenceStart + "\n1.2.3.4 example.com\n", "missing end marker"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseFileContent(tt.raw)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("parseFileContent error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseEntries(t *testing.T) {
	lines := []string{
		"1.2.3.4 Example.com # edgespoof:target=e1.a.akamaiedge-staging.net|mode=staging|ts=2026-11-07T05:00:00Z",
		"2606:4700::1 example.com # edgespoof:target=e1.a.akamaiedge-staging.net|mode=staging|ts=2026-11-07T05:00:00Z",
		"",
		"# plain comment",
		"malformed-line",
		"5.6.7.8 other.com",
		"9.9.9.9 third.com # unrelated comment",
	}
	want := []Entry{
		{
			Domain:      "example.com",
			IPv4:        "1.2.3.4",
			IPv6:        "2606:4700::1",
			TargetCNAME: "e1.a.akamaiedge-staging.net",
			Staging:     true,
			UpdatedAt:   "2026-11-07T05:00:00Z",
		},
		{Domain: "other.com", IPv4: "5.6.7.8"},
		{Domain: "third.com", IPv4: "9.9.9.9"},
	}

	got := parseEntries(lines)
	if !slices.Equal(got, want) {
		t.Errorf("parseEntries\n got %+v\nwant %+v", got, want)
	}
}

func TestFormatFileContentRoundTrip(t *testing.T) {
	entries := []Entry{
		{
			Domain:      "example.com",
			IPv4:        "1.2.3.4",
			IPv6:        "2606:4700::1",
			TargetCNAME: "e1.a.akamaiedge.net",
			UpdatedAt:   "2026-11-07T05:00:00Z",
		},
		{
			Domain:      "staging.example.com",
			IPv4:        "5.6.7.8",
			TargetCNAME: "e2.a.akamaiedge-staging.net",
			Staging:     true,
			UpdatedAt:   "2026-11-07T06:00:00Z",
		},
		{Domain: "v6.example.com", IPv6: "2606:4700::2"},
	}
	original := "127.0.0.1 localhost\n"

	sections, err := parseFileContent(original)
	if err != nil {
		t.Fatal(err)
	}
	formatted := formatFileContent(sections, entries)

	reparsed, err := parseFileContent(formatted)
	if err != nil {
		t.Fatalf("formatted content does not parse: %v\n%s", err, formatted)
	}
	if reparsed.before != original {
		t.Errorf("content before block = %q, want %q", reparsed.before, original)
	}

	got := parseEntries(contentLines(reparsed.block))
	if !slices.Equal(got, entries) {
		t.Errorf("round trip entries\n got %+v\nwant %+v", got, entries)
	}

	if again := formatFileContent(reparsed, got); again != formatted {
		t.Errorf("second format changed content\nfirst:\n%s\nsecond:\n%s", formatted, again)
	}
}

func TestFormatFileContent(t *testing.T) {
	entry := Entry{Domain: "example.com", IPv4: "1.2.3.4"}

	t.Run("adds newline before block", func(t *testing.T) {
		sections, _ := parseFileContent("127.0.0.1 localhost")
		got := formatFileContent(sections, []Entry{entry})
		if !strings.HasPrefix(got, "127.0.0.1 localhost\n"+FenceStart+"\n") {
			t.Errorf("block not on its own line:\n%s", got)
		}
	})

	t.Run("no entries removes block", func(t *testing.T) {
		raw := "127.0.0.1 localhost\n" + FenceStart + "\n1.2.3.4 example.com\n" + FenceEnd + "\n# after\n"
		sections, _ := parseFileContent(raw)
		got := formatFileContent(sections, nil)
		want := "127.0.0.1 localhost\n# after\n"
		if got != want {
			t.Errorf("formatFileContent = %q, want %q", got, want)
		}
	})

	t.Run("keeps crlf line endings", func(t *testing.T) {
		sections, _ := parseFileContent("127.0.0.1 localhost\r\n")
		got := formatFileContent(sections, []Entry{entry})
		lineFeeds := strings.Count(got, "\n")
		crlfs := strings.Count(got, "\r\n")
		if lineFeeds != crlfs {
			t.Errorf("found %d line feeds but %d CRLFs:\n%q", lineFeeds, crlfs, got)
		}
	})
}
