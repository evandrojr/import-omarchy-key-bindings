// Package planner resolves the Omarchy mapping against the live Ryoku bind
// catalog. It is pure: it never runs a command or writes a file, so the same
// plan can be printed, tested and then applied.
package planner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/evandrojr/import-omarchy-key-bindings/internal/chord"
	"github.com/evandrojr/import-omarchy-key-bindings/internal/keymap"
	"github.com/evandrojr/import-omarchy-key-bindings/internal/ryoku"
)

// OwnerKind says who holds a chord right now.
type OwnerKind string

const (
	OwnerCatalog OwnerKind = "catalog"
	OwnerManaged OwnerKind = "managed"
	OwnerManual  OwnerKind = "manual"
)

// Owner is the current holder of a chord.
type Owner struct {
	Kind    OwnerKind
	ID      string
	Label   string
	Default string
}

// Input is everything the planner needs.
type Input struct {
	Actions []keymap.Action
	Catalog ryoku.Catalog
	Rebinds map[string]string
	Managed []string // chords bound inside the generated block
	Manual  []string // chords bound by hand in user.lua
	Adopt   bool     // take over manual binds that sit on a mapped chord
}

// Move is one default-chord -> chosen-chord store change.
type Move struct {
	BindID  string
	Label   string
	Default string
	From    string
	To      string
}

// Add is one bind for the generated block.
type Add struct {
	ActionID string
	Label    string
	Chord    string
	Lua      string
}

// Row is one line of the plan.
type Row struct {
	Action keymap.Action
	Now    string
	Result string
	Detail string
}

// Result values.
const (
	ResultSame     = "same"
	ResultMoved    = "moved"
	ResultNew      = "new"
	ResultConflict = "conflict resolved"
	ResultSkipped  = "not ported"
	ResultError    = "error"
)

// Plan is the complete, deterministic change set.
type Plan struct {
	Rows     []Row
	Moves    []Move
	Adds     []Add
	Adopted  []string
	Warnings []string
	Targets  map[string]string // normalized chord -> label
}

type state struct {
	actions  []keymap.Action
	byID     map[string]ryoku.Bind
	at       map[string]string // bind id -> current chord
	occ      map[string]Owner  // normalized chord -> owner
	rebinds  map[string]string
	moves    []Move
	adds     []Add
	warnings []string
}

// Build resolves the mapping. Errors are reported per row; the returned error
// is reserved for input that makes planning impossible.
func Build(in Input) (Plan, error) {
	s := &state{
		actions: in.Actions,
		byID:    map[string]ryoku.Bind{},
		at:      map[string]string{},
		occ:     map[string]Owner{},
		rebinds: map[string]string{},
	}
	for k, v := range in.Rebinds {
		s.rebinds[k] = v
	}

	// Catalog binds, skipping the legend-only rows the compositor never binds.
	for _, b := range in.Catalog.All() {
		s.byID[b.ID] = b
		if b.Unhonored != "" || b.Chord() == "" {
			continue
		}
		eff := b.Chord()
		if r, ok := s.rebinds[b.Chord()]; ok && r != "" {
			eff = r
		}
		s.at[b.ID] = eff
		n := chord.Normalize(eff)
		if prev, ok := s.occ[n]; ok {
			s.warnings = append(s.warnings, fmt.Sprintf("catalog collision on %s: %s and %s", chord.Display(eff), prev.Label, b.Label))
			continue
		}
		s.occ[n] = Owner{Kind: OwnerCatalog, ID: b.ID, Label: b.Label, Default: b.Chord()}
	}

	// Chords the plan itself will own; --adopt may reclaim hand-written ones.
	claimed := map[string]bool{}
	for _, a := range in.Actions {
		if a.Kind == keymap.Add || a.Kind == keymap.Align {
			claimed[chord.Normalize(a.Omarchy)] = true
		}
	}
	var adopted []string
	for _, c := range in.Manual {
		if in.Adopt && claimed[chord.Normalize(c)] {
			adopted = append(adopted, c)
			s.warnings = append(s.warnings, fmt.Sprintf("adopting manual bind on %s into the managed block", chord.Display(c)))
			continue
		}
		n := chord.Normalize(c)
		if prev, ok := s.occ[n]; ok {
			s.warnings = append(s.warnings, fmt.Sprintf("manual bind on %s shadows %s; it is treated as protected", chord.Display(c), prev.Label))
		}
		s.occ[n] = Owner{Kind: OwnerManual, Label: "user.lua", Default: c}
	}
	for _, c := range in.Managed {
		n := chord.Normalize(c)
		if prev, ok := s.occ[n]; ok {
			s.warnings = append(s.warnings, fmt.Sprintf("managed bind on %s duplicates %s", chord.Display(c), prev.Label))
			continue
		}
		s.occ[n] = Owner{Kind: OwnerManaged, Label: "generated block"}
	}

	plan := Plan{Targets: map[string]string{}, Warnings: s.warnings}
	for _, a := range in.Actions {
		row := Row{Action: a}
		switch a.Kind {
		case keymap.Keep:
			row.Now = chord.Display(a.Omarchy)
			row.Result = ResultSame
			row.Detail = "already matches"
		case keymap.Skip:
			row.Now = "-"
			row.Result = ResultSkipped
			row.Detail = a.Reason
		case keymap.Align:
			bind, ok := s.byID[a.Bind]
			if !ok {
				row.Result = ResultError
				row.Detail = fmt.Sprintf("unknown catalog bind %q", a.Bind)
				break
			}
			cur := s.at[bind.ID]
			row.Now = cur
			if chord.Equal(cur, a.Omarchy) {
				row.Result = ResultSame
				row.Detail = "already matches"
				break
			}
			moved, err := s.free(a.Omarchy, bind.ID)
			if err != nil {
				row.Result = ResultError
				row.Detail = err.Error()
				break
			}
			s.move(bind.ID, a.Omarchy)
			row.Result = ResultMoved
			if moved > 0 {
				row.Result = ResultConflict
			}
			row.Detail = fmt.Sprintf("%s -> %s", chord.Display(cur), chord.Display(a.Omarchy))
		case keymap.Add:
			desired := a.Omarchy
			row.Now = "free"
			if keymap.IsProtected(desired) {
				row.Result = ResultError
				row.Detail = "chord is protected (Ryoku Settings / launcher)"
				break
			}
			if owner, ok := s.occ[chord.Normalize(desired)]; ok {
				row.Now = owner.Label
				if owner.Kind == OwnerManaged {
					s.add(a)
					row.Result = ResultSame
					row.Detail = "already in the generated block"
					break
				}
			}
			moved, err := s.free(desired, "add:"+a.ID)
			if err != nil {
				row.Result = ResultError
				row.Detail = err.Error()
				break
			}
			s.add(a)
			row.Result = ResultNew
			if moved > 0 {
				row.Result = ResultConflict
			}
			row.Detail = desired
		}
		if row.Result != ResultSkipped && row.Result != ResultError {
			plan.Targets[chord.Normalize(a.Omarchy)] = a.Label
		}
		plan.Rows = append(plan.Rows, row)
	}
	for _, m := range s.moves {
		plan.Targets[chord.Normalize(m.To)] = m.Label
	}
	plan.Moves = s.moves
	plan.Adds = s.adds
	sort.Strings(adopted)
	plan.Adopted = adopted
	plan.Warnings = s.warnings
	return plan, nil
}

// free clears a chord by moving a movable catalog owner to a fallback. It
// returns how many binds it moved. Chains are resolved by processing the
// caller's target first, then the owner's own move.
func (s *state) free(want, requester string) (int, error) {
	owner, ok := s.occ[chord.Normalize(want)]
	if !ok {
		return 0, nil
	}
	if owner.Kind != OwnerCatalog {
		return 0, fmt.Errorf("%s is held by %s; refusing to move it", chord.Display(want), owner.Label)
	}
	if owner.ID == requester {
		return 0, nil
	}
	bind, ok := s.byID[owner.ID]
	if !ok {
		return 0, fmt.Errorf("%s is held by an unknown bind %q", chord.Display(want), owner.ID)
	}
	if bind.Locked || !bind.Rebindable {
		return 0, fmt.Errorf("%s is held by locked bind %q", chord.Display(want), bind.Label)
	}
	if keymap.IsProtected(bind.Chord()) {
		return 0, fmt.Errorf("%s is held by protected bind %q", chord.Display(want), bind.Label)
	}
	for _, cand := range candidates(bind.ID, s.at[bind.ID], want) {
		if chord.Equal(cand, want) || chord.Equal(cand, s.at[bind.ID]) {
			continue
		}
		if keymap.IsProtected(cand) {
			continue
		}
		if _, busy := s.occ[chord.Normalize(cand)]; busy {
			continue
		}
		s.move(bind.ID, cand)
		return 1, nil
	}
	return 0, fmt.Errorf("no free fallback for %q (held on %s)", bind.Label, chord.Display(want))
}

func candidates(bindID, current, want string) []string {
	var out []string
	if fb := keymap.Fallback(bindID); fb != "" {
		out = append(out, fb)
	}
	base := current
	if base == "" {
		base = want
	}
	c := chord.Parse(base)
	existing := c.Mods
	sets := [][]string{
		{"ALT"}, {"CTRL"}, {"SHIFT"},
		{"ALT", "CTRL"}, {"ALT", "SHIFT"}, {"CTRL", "SHIFT"},
		{"ALT", "CTRL", "SHIFT"},
	}
	for _, add := range sets {
		mods := append([]string{}, existing...)
		mods = append(mods, add...)
		out = append(out, chord.Display(strings.Join(append(mods, c.Key), " + ")))
	}
	return out
}

func (s *state) move(bindID, to string) {
	cur := s.at[bindID]
	delete(s.occ, chord.Normalize(cur))
	bind := s.byID[bindID]
	s.at[bindID] = to
	s.occ[chord.Normalize(to)] = Owner{Kind: OwnerCatalog, ID: bindID, Label: bind.Label, Default: bind.Chord()}
	s.rebinds[bind.Chord()] = to
	s.moves = append(s.moves, Move{
		BindID:  bindID,
		Label:   bind.Label,
		Default: bind.Chord(),
		From:    cur,
		To:      to,
	})
}

func (s *state) add(a keymap.Action) {
	for _, existing := range s.adds {
		if existing.ActionID == a.ID {
			return
		}
	}
	s.adds = append(s.adds, Add{ActionID: a.ID, Label: a.Label, Chord: a.Omarchy, Lua: a.Lua})
	s.occ[chord.Normalize(a.Omarchy)] = Owner{Kind: OwnerManaged, ID: a.ID, Label: a.Label}
}
