package userlua

import (
	"strings"
	"testing"
)

const sample = `-- header
-- hl.bind("SUPER + Z", hl.dsp.exec_cmd("commented"))
hl.bind("SUPER + S", hl.dsp.exec_cmd("scratch"))

-- >>> import-omarchy-key-bindings >>>
hl.bind("SUPER + C", iokb_clipboard("CTRL", "C"))
-- <<< import-omarchy-key-bindings <<<

hl.bind("SUPER + B", hl.dsp.exec_cmd("browser"))
`

func TestBindingsClassification(t *testing.T) {
	managed, manual := Bindings(sample)
	if len(managed) != 1 || managed[0] != "SUPER + C" {
		t.Fatalf("managed = %v", managed)
	}
	if len(manual) != 2 || manual[0] != "SUPER + S" || manual[1] != "SUPER + B" {
		t.Fatalf("manual = %v", manual)
	}
}

func TestUpsertReplacesOnlyBlock(t *testing.T) {
	out := Upsert(sample, "hl.bind(\"SUPER + X\", nil)")
	if !strings.Contains(out, `hl.bind("SUPER + S", hl.dsp.exec_cmd("scratch"))`) {
		t.Fatal("manual bind lost")
	}
	if !strings.Contains(out, `hl.bind("SUPER + B"`) {
		t.Fatal("trailing bind lost")
	}
	if strings.Contains(out, `SUPER + C`) {
		t.Fatal("old block content kept")
	}
	if strings.Count(out, Begin) != 1 || strings.Count(out, End) != 1 {
		t.Fatal("markers duplicated")
	}
	if !strings.Contains(out, `hl.bind("SUPER + X", nil)`) {
		t.Fatal("new block missing")
	}
}

func TestUpsertIsIdempotent(t *testing.T) {
	block := "hl.bind(\"SUPER + C\", iokb_clipboard(\"CTRL\", \"C\"))\nhl.bind(\"SUPER + V\", iokb_clipboard(\"CTRL\", \"V\"))"
	once := Upsert("-- header\n", block)
	twice := Upsert(once, block)
	if once != twice {
		t.Fatalf("not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
}

func TestUpsertAppendsWhenMissing(t *testing.T) {
	out := Upsert("-- header\n", "hl.bind(\"SUPER + C\", nil)")
	if !strings.HasPrefix(out, "-- header") {
		t.Fatal("header lost")
	}
	if !strings.Contains(out, Begin) || !strings.Contains(out, End) {
		t.Fatal("markers missing")
	}
}

func TestRemoveBindsOnlyOutsideBlock(t *testing.T) {
	out, removed := RemoveBinds(sample, map[string]bool{"SUPER+s": true, "SUPER+b": true})
	if len(removed) != 2 {
		t.Fatalf("removed = %v", removed)
	}
	if strings.Contains(out, `hl.bind("SUPER + S"`) {
		t.Fatal("S should be removed")
	}
	if !strings.Contains(out, `hl.bind("SUPER + C"`) {
		t.Fatal("C is inside the block and must stay")
	}
	if !strings.Contains(out, `-- hl.bind("SUPER + Z"`) {
		t.Fatal("commented example must stay")
	}
}
