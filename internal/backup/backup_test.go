package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateLatestRestore(t *testing.T) {
	state := t.TempDir()
	live := filepath.Join(t.TempDir(), "user.lua")
	if err := os.WriteFile(live, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Create(state, []Entry{{Source: live, Rel: "hypr/user.lua"}}, map[string]string{"why": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b.Dir, "hypr", "user.lua")); err != nil {
		t.Fatal("snapshot missing")
	}
	if err := os.WriteFile(live, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	latest, err := Latest(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := latest.Restore(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(live)
	if string(got) != "original" {
		t.Fatalf("restored %q", got)
	}
}
