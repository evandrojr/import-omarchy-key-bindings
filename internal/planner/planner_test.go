package planner

import (
	"encoding/json"
	"testing"

	"github.com/evandrojr/import-omarchy-key-bindings/internal/chord"
	"github.com/evandrojr/import-omarchy-key-bindings/internal/keymap"
	"github.com/evandrojr/import-omarchy-key-bindings/internal/ryoku"
)

func catalog(t *testing.T, binds ...ryoku.Bind) ryoku.Catalog {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"categories": []any{map[string]any{"name": "test", "binds": binds}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var c ryoku.Catalog
	if err := json.Unmarshal(payload, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func baseCatalog(t *testing.T) ryoku.Catalog {
	return catalog(t,
		ryoku.Bind{ID: "shell.lock", Label: "Lock", Combo: "SUPER + L", Rebindable: true},
		ryoku.Bind{ID: "column.tabbed", Label: "Group", Combo: "SUPER + T", Rebindable: true},
		ryoku.Bind{ID: "window.float", Label: "Float", Combo: "SUPER + A", Rebindable: true},
		ryoku.Bind{ID: "column.center", Label: "Centre", Combo: "SUPER + C", Rebindable: true},
		ryoku.Bind{ID: "shell.clipboard", Label: "Clipboard", Combo: "SUPER + V", Rebindable: true},
		ryoku.Bind{ID: "workspace.overview", Label: "Overview", Combo: "SUPER + Tab", Rebindable: true},
		ryoku.Bind{ID: "shell.voice", Label: "Voice", Combo: "SUPER + grave", Rebindable: true},
		ryoku.Bind{ID: "workspace.scratchpad", Label: "Scratch", Combo: "SUPER + ALT + H", Rebindable: true},
		ryoku.Bind{ID: "shell.settings", Label: "Settings", Combo: "SUPER + comma", Rebindable: true},
		ryoku.Bind{ID: "media.mute", Label: "Mute", Combo: "XF86AudioMute", Rebindable: false, Locked: true},
	)
}

func movesByBind(p Plan) map[string]Move {
	out := map[string]Move{}
	for _, m := range p.Moves {
		out[m.BindID] = m
	}
	return out
}

func TestConflictChainOrdersOccupantFirst(t *testing.T) {
	plan, err := Build(Input{
		Actions: []keymap.Action{
			{ID: "group", Kind: keymap.Align, Omarchy: "SUPER + G", Bind: "column.tabbed"},
			{ID: "float", Kind: keymap.Align, Omarchy: "SUPER + T", Bind: "window.float"},
		},
		Catalog: baseCatalog(t),
		Rebinds: map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Moves) != 2 {
		t.Fatalf("moves = %+v", plan.Moves)
	}
	if plan.Moves[0].BindID != "column.tabbed" || plan.Moves[0].To != "SUPER + G" {
		t.Fatalf("first move should free T first: %+v", plan.Moves[0])
	}
	if plan.Moves[1].BindID != "window.float" || plan.Moves[1].To != "SUPER + T" {
		t.Fatalf("second move: %+v", plan.Moves[1])
	}
	if _, ok := plan.Targets[chord.Normalize("SUPER + G")]; !ok {
		t.Fatal("G missing from targets")
	}
}

func TestAddMovesOccupantToCuratedFallback(t *testing.T) {
	plan, err := Build(Input{
		Actions: []keymap.Action{
			{ID: "copy", Kind: keymap.Add, Omarchy: "SUPER + C", Lua: "iokb_clipboard(\"CTRL\", \"C\", \"CTRL SHIFT\", \"C\")"},
			{ID: "paste", Kind: keymap.Add, Omarchy: "SUPER + V", Lua: "iokb_clipboard(\"CTRL\", \"V\")"},
		},
		Catalog: baseCatalog(t),
		Rebinds: map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	m := movesByBind(plan)
	if got := m["column.center"].To; got != "SUPER + ALT + C" {
		t.Fatalf("centre fallback = %q", got)
	}
	if got := m["shell.clipboard"].To; got != "SUPER + CTRL + V" {
		t.Fatalf("clipboard fallback = %q", got)
	}
	if len(plan.Adds) != 2 {
		t.Fatalf("adds = %+v", plan.Adds)
	}
}

func TestScratchChainMovesVoice(t *testing.T) {
	plan, err := Build(Input{
		Actions: []keymap.Action{
			{ID: "scratch", Kind: keymap.Align, Omarchy: "SUPER + grave", Bind: "workspace.scratchpad"},
		},
		Catalog: baseCatalog(t),
		Rebinds: map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	m := movesByBind(plan)
	if got := m["shell.voice"].To; got != "SUPER + ALT + grave" {
		t.Fatalf("voice fallback = %q", got)
	}
	if got := m["workspace.scratchpad"].To; got != "SUPER + grave" {
		t.Fatalf("scratch target = %q", got)
	}
}

func TestIdempotent(t *testing.T) {
	actions := []keymap.Action{
		{ID: "group", Kind: keymap.Align, Omarchy: "SUPER + G", Bind: "column.tabbed"},
		{ID: "float", Kind: keymap.Align, Omarchy: "SUPER + T", Bind: "window.float"},
		{ID: "copy", Kind: keymap.Add, Omarchy: "SUPER + C", Lua: "iokb_clipboard(\"CTRL\", \"C\", \"CTRL SHIFT\", \"C\")"},
	}
	first, err := Build(Input{Actions: actions, Catalog: baseCatalog(t), Rebinds: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	rebinds := map[string]string{}
	for _, m := range first.Moves {
		rebinds[m.Default] = m.To
	}
	var managed []string
	for _, a := range first.Adds {
		managed = append(managed, a.Chord)
	}
	second, err := Build(Input{Actions: actions, Catalog: baseCatalog(t), Rebinds: rebinds, Managed: managed})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Moves) != 0 {
		t.Fatalf("second plan still moves: %+v", second.Moves)
	}
	for _, row := range second.Rows {
		if row.Result == ResultError || row.Result == ResultMoved {
			t.Fatalf("row not idempotent: %+v", row)
		}
	}
}

func TestProtectedChordNeverClaimed(t *testing.T) {
	plan, err := Build(Input{
		Actions: []keymap.Action{
			{ID: "settings", Kind: keymap.Add, Omarchy: "SUPER + comma", Lua: "hl.dsp.exec_cmd(\"x\")"},
		},
		Catalog: baseCatalog(t),
		Rebinds: map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Rows[0].Result != ResultError {
		t.Fatalf("expected error, got %+v", plan.Rows[0])
	}
}

func TestLockedBindCannotBeMoved(t *testing.T) {
	plan, err := Build(Input{
		Actions: []keymap.Action{
			{ID: "mute", Kind: keymap.Add, Omarchy: "XF86AudioMute", Lua: "hl.dsp.exec_cmd(\"x\")"},
		},
		Catalog: baseCatalog(t),
		Rebinds: map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Rows[0].Result != ResultError {
		t.Fatalf("expected error, got %+v", plan.Rows[0])
	}
}

func TestManualBindIsProtectedUnlessAdopted(t *testing.T) {
	actions := []keymap.Action{
		{ID: "copy", Kind: keymap.Add, Omarchy: "SUPER + C", Lua: "iokb_clipboard(\"CTRL\", \"C\", \"CTRL SHIFT\", \"C\")"},
	}
	plan, _ := Build(Input{Actions: actions, Catalog: baseCatalog(t), Rebinds: map[string]string{}, Manual: []string{"SUPER + C"}})
	if plan.Rows[0].Result != ResultError {
		t.Fatalf("manual should block, got %+v", plan.Rows[0])
	}
	plan, _ = Build(Input{Actions: actions, Catalog: baseCatalog(t), Rebinds: map[string]string{}, Manual: []string{"SUPER + C"}, Adopt: true})
	if plan.Rows[0].Result != ResultConflict {
		t.Fatalf("adopted chord still displaces the catalog centre, got %+v", plan.Rows[0])
	}
	if len(plan.Adopted) != 1 || plan.Adopted[0] != "SUPER + C" {
		t.Fatalf("adopted = %+v", plan.Adopted)
	}
}

func TestSkipAndKeepRows(t *testing.T) {
	plan, _ := Build(Input{
		Actions: []keymap.Action{
			{ID: "launcher", Kind: keymap.Keep, Omarchy: "SUPER + Space"},
			{ID: "music", Kind: keymap.Skip, Omarchy: "SUPER + SHIFT + M", Reason: "Spotify absent"},
		},
		Catalog: baseCatalog(t),
		Rebinds: map[string]string{},
	})
	if plan.Rows[0].Result != ResultSame || plan.Rows[1].Result != ResultSkipped {
		t.Fatalf("rows = %+v", plan.Rows)
	}
	if _, ok := plan.Targets[chord.Normalize("SUPER + SHIFT + M")]; ok {
		t.Fatal("skip must not be a target")
	}
}
