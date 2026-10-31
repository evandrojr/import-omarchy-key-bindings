package omarchy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirParsesLabelledBinds(t *testing.T) {
	dir := t.TempDir()
	src := `-- comment
o.bind("SUPER + C", "Universal copy", send_shortcut_once("CTRL", "C"))
o.bind('SUPER + V', 'Universal paste', nil)
o.bind("SUPER + A", "Select all", nil)
`
	if err := os.WriteFile(filepath.Join(dir, "clipboard.lua"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Dir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := d.ChordFor("universal copy"); got != "SUPER + C" {
		t.Fatalf("copy = %q", got)
	}
	if got, _ := d.ChordFor("Universal Paste"); got != "SUPER + V" {
		t.Fatalf("paste = %q", got)
	}
}
