// Package backup snapshots the files the tool is allowed to change and can put
// them back. Restoring is a plain file copy plus a reload, so it works even if
// the tool itself is broken.
package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Entry maps a live file to its path inside the backup.
type Entry struct {
	Source string `json:"source"`
	Rel    string `json:"rel"`
}

// Manifest describes one backup.
type Manifest struct {
	ID      string          `json:"id"`
	Created time.Time       `json:"created"`
	Entries []Entry         `json:"entries"`
	Meta    json.RawMessage `json:"meta,omitempty"`
}

// Backup is a stored snapshot.
type Backup struct {
	Manifest Manifest
	Dir      string
}

// Create copies each entry under <stateDir>/backups/<timestamp>/.
func Create(stateDir string, entries []Entry, meta any) (Backup, error) {
	id := time.Now().Format("20060102-150405")
	root := filepath.Join(stateDir, "backups")
	dir := filepath.Join(root, id)
	if _, err := os.Stat(dir); err == nil {
		id += fmt.Sprintf("-%d", time.Now().UnixNano()%1000)
		dir = filepath.Join(root, id)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Backup{}, err
	}
	m := Manifest{ID: id, Created: time.Now(), Entries: entries}
	if meta != nil {
		raw, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return Backup{}, err
		}
		m.Meta = raw
	}
	for _, e := range entries {
		dst := filepath.Join(dir, e.Rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return Backup{}, err
		}
		if err := copyFile(e.Source, dst); err != nil {
			return Backup{}, err
		}
	}
	if err := writeManifest(dir, m); err != nil {
		return Backup{}, err
	}
	return Backup{Manifest: m, Dir: dir}, nil
}

func writeManifest(dir string, m Manifest) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), append(raw, '\n'), 0o644)
}

// Latest returns the most recent backup under stateDir.
func Latest(stateDir string) (Backup, error) {
	root := filepath.Join(stateDir, "backups")
	entries, err := os.ReadDir(root)
	if err != nil {
		return Backup{}, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return Backup{}, fmt.Errorf("no backups under %s", root)
	}
	sort.Strings(names)
	return Open(filepath.Join(root, names[len(names)-1]))
}

// Open loads a backup directory.
func Open(dir string) (Backup, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Backup{}, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Backup{}, err
	}
	return Backup{Manifest: m, Dir: dir}, nil
}

// Restore copies every snapshotted file back to its live path.
func (b Backup) Restore() error {
	for _, e := range b.Manifest.Entries {
		src := filepath.Join(b.Dir, e.Rel)
		if err := copyFile(src, e.Source); err != nil {
			return fmt.Errorf("restore %s: %w", e.Source, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o644)
}
