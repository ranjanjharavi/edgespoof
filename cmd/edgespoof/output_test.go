package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAlignUnderLabel(t *testing.T) {
	tests := []struct {
		name    string
		label   string
		message string
		want    string
	}{
		{"single line unchanged", "✖ Error:", "boom", "boom"},
		{"joined error", "✖ Error:", "A failed\nAAAA failed", "A failed\n         AAAA failed"},
		{"warning label width", "! Warning:", "one\ntwo\nthree", "one\n           two\n           three"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := alignUnderLabel(tt.label, tt.message); got != tt.want {
				t.Errorf("alignUnderLabel(%q, %q) = %q, want %q", tt.label, tt.message, got, tt.want)
			}
		})
	}
}

func TestFormatUpdated(t *testing.T) {
	local := time.Local
	time.Local = time.FixedZone("IST", 5*60*60+30*60)
	t.Cleanup(func() { time.Local = local })

	tests := []struct {
		timestamp string
		want      string
	}{
		{"2026-11-07T05:00:00Z", "2026-11-07 10:30:00"},
		{"", "-"},
		{"not-a-time", "-"},
	}

	for _, tt := range tests {
		if got := formatUpdated(tt.timestamp); got != tt.want {
			t.Errorf("formatUpdated(%q) = %q, want %q", tt.timestamp, got, tt.want)
		}
	}
}

func TestPaletteForDisablesColor(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	t.Run("not a terminal", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if got := paletteFor(file); got != (palette{}) {
			t.Errorf("paletteFor(regular file) = %+v, want no colors", got)
		}
	})

	t.Run("NO_COLOR set", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		if got := paletteFor(os.Stdout); got != (palette{}) {
			t.Errorf("paletteFor with NO_COLOR = %+v, want no colors", got)
		}
	})
}
