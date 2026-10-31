package render

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evandrojr/import-omarchy-key-bindings/internal/planner"
)

func TestBlockParsesWithLuac(t *testing.T) {
	if _, err := exec.LookPath("luac"); err != nil {
		t.Skip("luac not installed")
	}
	block := Block([]planner.Add{
		{ActionID: "copy", Label: "Copy", Chord: "SUPER + C", Lua: `iokb_clipboard("CTRL", "C", "CTRL SHIFT", "C")`},
		{ActionID: "btop", Label: "btop", Chord: "SUPER + SHIFT + T", Lua: `hl.dsp.exec_cmd("kitty -e btop")`},
	})
	if strings.Contains(block, "SUPER + C\",") == false {
		t.Fatal("bind line missing")
	}
	path := filepath.Join(t.TempDir(), "block.lua")
	if err := os.WriteFile(path, []byte(block), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("luac", "-p", path).CombinedOutput(); err != nil {
		t.Fatalf("luac -p: %v\n%s", err, out)
	}
}
