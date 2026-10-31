// Package userlua owns the managed block inside ~/.config/hypr/user.lua.
// Everything outside the markers belongs to the user and is never rewritten.
package userlua

import (
	"regexp"
	"strings"

	"github.com/evandrojr/import-omarchy-key-bindings/internal/chord"
)

const (
	// Begin and End delimit the generated block.
	Begin = "-- >>> import-omarchy-key-bindings >>>"
	End   = "-- <<< import-omarchy-key-bindings <<<"
)

var bindRe = regexp.MustCompile(`hl\.bind\(\s*["']([^"']+)["']`)

// Split returns the text before the block, its body and the text after it.
func Split(src string) (before, block, after string, found bool) {
	lines := strings.Split(src, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == Begin {
			start = i
			break
		}
	}
	if start < 0 {
		return src, "", "", false
	}
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == End {
			return strings.Join(lines[:start], "\n"),
				strings.Join(lines[start+1:i], "\n"),
				strings.Join(lines[i+1:], "\n"),
				true
		}
	}
	// Unterminated block: treat everything after the marker as the block.
	return strings.Join(lines[:start], "\n"), strings.Join(lines[start+1:], "\n"), "", true
}

// Upsert replaces the managed block, or appends one when absent. The text
// outside the markers is returned byte-for-byte apart from newline joining.
func Upsert(src, block string) string {
	before, _, after, found := Split(src)
	block = strings.TrimRight(block, "\n")
	if found {
		out := strings.TrimRight(before, "\n")
		if out != "" {
			out += "\n\n"
		}
		out += Begin + "\n"
		if block != "" {
			out += block + "\n"
		}
		out += End
		if strings.TrimSpace(after) != "" {
			out += "\n" + strings.TrimLeft(after, "\n")
		}
		return out + "\n"
	}
	out := strings.TrimRight(src, "\n")
	if out != "" {
		out += "\n\n"
	}
	out += Begin + "\n"
	if block != "" {
		out += block + "\n"
	}
	out += End + "\n"
	return out
}

// Bindings classifies the hl.bind chords inside and outside the managed block.
// Commented lines are ignored, which matters because the seeded user.lua has
// examples in its header.
func Bindings(src string) (managed, manual []string) {
	inBlock := false
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case Begin:
			inBlock = true
			continue
		case End:
			inBlock = false
			continue
		}
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		m := bindRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if inBlock {
			managed = append(managed, m[1])
		} else {
			manual = append(manual, m[1])
		}
	}
	return managed, manual
}

// RemoveBinds deletes single-line hl.bind statements outside the managed block
// whose chord is in chords. It is used by --adopt to take over an earlier
// hand-written Omarchy section. Returns the new source and the removed chords.
func RemoveBinds(src string, chords map[string]bool) (string, []string) {
	if len(chords) == 0 {
		return src, nil
	}
	var out []string
	var removed []string
	inBlock := false
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case Begin:
			inBlock = true
			out = append(out, line)
			continue
		case End:
			inBlock = false
			out = append(out, line)
			continue
		}
		if !inBlock && !strings.HasPrefix(trimmed, "--") {
			if m := bindRe.FindStringSubmatch(line); m != nil && chords[chord.Normalize(m[1])] {
				removed = append(removed, m[1])
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), removed
}
