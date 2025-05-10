package hosts

import (
	"errors"
	"fmt"
	"strings"
)

const (
	FenceStart = "# --- EDGESPOOF START ---"
	FenceEnd   = "# --- EDGESPOOF END ---"
)

type fileSections struct {
	before   string
	block    string
	after    string
	newline  string
	hasBlock bool
}

func parseFileContent(raw string) (fileSections, error) {
	sections := fileSections{
		before:  raw,
		newline: lineEnding(raw),
	}

	blockStart := 0
	foundEnd := false
	offset := 0
	for _, line := range strings.SplitAfter(raw, "\n") {
		switch strings.TrimSpace(line) {
		case FenceStart:
			if sections.hasBlock {
				return fileSections{}, errors.New("invalid edgespoof block: duplicate start marker")
			}

			sections.before = raw[:offset]
			sections.newline = lineEnding(line)
			sections.hasBlock = true
			blockStart = offset + len(line)

		case FenceEnd:
			if !sections.hasBlock {
				return fileSections{}, errors.New("invalid edgespoof block: end marker before start marker")
			}
			if foundEnd {
				return fileSections{}, errors.New("invalid edgespoof block: duplicate end marker")
			}

			sections.block = raw[blockStart:offset]
			sections.after = raw[offset+len(line):]
			foundEnd = true
		}
		offset += len(line)
	}

	if sections.hasBlock && !foundEnd {
		return fileSections{}, errors.New("invalid edgespoof block: missing end marker")
	}

	return sections, nil
}

func lineEnding(raw string) string {
	newlineIndex := strings.IndexByte(raw, '\n')
	if newlineIndex > 0 && raw[newlineIndex-1] == '\r' {
		return "\r\n"
	}
	return "\n"
}

func contentLines(raw string) []string {
	if raw == "" {
		return nil
	}
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	return strings.Split(normalized, "\n")
}

// Merge address families without changing domain order
func parseEntries(lines []string) []Entry {
	byDomain := make(map[string]*Entry)
	var order []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		hostPart, comment, hasComment := strings.Cut(trimmed, "#")
		hostFields := strings.Fields(hostPart)
		if len(hostFields) < 2 {
			continue
		}

		ip := hostFields[0]
		domain := strings.ToLower(hostFields[1])

		ent, exists := byDomain[domain]
		if !exists {
			ent = &Entry{Domain: domain}
			byDomain[domain] = ent
			order = append(order, domain)
		}

		if strings.Contains(ip, ":") {
			ent.IPv6 = ip
		} else {
			ent.IPv4 = ip
		}

		if hasComment {
			parseCommentMetadata(comment, ent)
		}
	}

	var result []Entry
	for _, domain := range order {
		result = append(result, *byDomain[domain])
	}
	return result
}

func parseCommentMetadata(rawComment string, ent *Entry) {
	meta := strings.TrimSpace(rawComment)
	meta, found := strings.CutPrefix(meta, "edgespoof:")
	if !found {
		return
	}

	for _, kv := range strings.Split(meta, "|") {
		key, value, found := strings.Cut(kv, "=")
		if !found {
			continue
		}

		switch key {
		case "target":
			ent.TargetCNAME = value
		case "mode":
			ent.Staging = value == "staging"
		case "ts":
			ent.UpdatedAt = value
		}
	}
}

func formatFileContent(sections fileSections, entries []Entry) string {
	if len(entries) == 0 {
		return sections.before + sections.after
	}

	var block strings.Builder
	block.WriteString(FenceStart)
	block.WriteString(sections.newline)

	for _, entry := range entries {
		mode := "edge"
		if entry.Staging {
			mode = "staging"
		}

		meta := fmt.Sprintf(
			"# edgespoof:target=%s|mode=%s|ts=%s",
			entry.TargetCNAME,
			mode,
			entry.UpdatedAt,
		)

		if entry.IPv4 != "" {
			fmt.Fprintf(&block, "%-28s %-32s %s%s", entry.IPv4, entry.Domain, meta, sections.newline)
		}

		if entry.IPv6 != "" {
			fmt.Fprintf(&block, "%-28s %-32s %s%s", entry.IPv6, entry.Domain, meta, sections.newline)
		}
	}
	block.WriteString(FenceEnd)
	block.WriteString(sections.newline)

	before := sections.before
	if !sections.hasBlock && before != "" && !strings.HasSuffix(before, "\n") {
		before += sections.newline
	}
	return before + block.String() + sections.after
}
