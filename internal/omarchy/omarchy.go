// Package omarchy optionally reads an Omarchy checkout so the curated table can
// follow upstream chord changes instead of going stale. It is best-effort: it
// understands the o.bind("CHORD", "Label", ...) form used by Omarchy's
// bindings/*.lua and ignores everything else.
package omarchy

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var bindRe = regexp.MustCompile(`(?:o|hl)\.bind\(\s*["']([^"']+)["']\s*,\s*["']([^"']+)["']`)

// Discovery is a label -> chord map found in Omarchy's config.
type Discovery struct {
	Source string
	Chords map[string]string // lower-case label -> chord
}

// Dir walks dir for *.lua and extracts labelled binds.
func Dir(dir string) (Discovery, error) {
	d := Discovery{Source: dir, Chords: map[string]string{}}
	info, err := os.Stat(dir)
	if err != nil {
		return d, err
	}
	if !info.IsDir() {
		return d, fmt.Errorf("%s is not a directory", dir)
	}
	err = filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || !strings.HasSuffix(path, ".lua") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range bindRe.FindAllStringSubmatch(string(raw), -1) {
			d.Chords[strings.ToLower(strings.TrimSpace(m[2]))] = m[1]
		}
		return nil
	})
	if err != nil {
		return d, err
	}
	if len(d.Chords) == 0 {
		return d, fmt.Errorf("no o.bind() chords found under %s", dir)
	}
	return d, nil
}

// ChordFor returns the chord Omarchy currently uses for a label.
func (d Discovery) ChordFor(label string) (string, bool) {
	c, ok := d.Chords[strings.ToLower(strings.TrimSpace(label))]
	return c, ok
}
