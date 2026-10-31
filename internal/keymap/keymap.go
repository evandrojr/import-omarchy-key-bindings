// Package keymap holds the curated Omarchy -> Ryoku mapping. The JSON is
// embedded so the binary works on a machine with no checkout; the table is the
// part that took the research, and it is deliberately data, not code.
package keymap

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/evandrojr/import-omarchy-key-bindings/internal/chord"
)

//go:embed keymap.json
var raw []byte

// Kinds of action in the table.
const (
	Keep  = "keep"  // both systems already agree; report only
	Align = "align" // move an existing Ryoku bind onto the Omarchy chord
	Add   = "add"   // add a new bind on a free chord
	Skip  = "skip"  // no equivalent in Ryoku; report only
)

// Action is one row of the mapping table.
type Action struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Kind         string `json:"kind"`
	Omarchy      string `json:"omarchy"`
	Bind         string `json:"bind,omitempty"`          // catalog id for align
	Lua          string `json:"lua,omitempty"`           // second argument for add
	OmarchyLabel string `json:"omarchy_label,omitempty"` // label in Omarchy's bindings.lua
	Reason       string `json:"reason,omitempty"`        // skip
	Protected    bool   `json:"protected,omitempty"`     // chord may never be moved
}

// Load parses and validates the embedded table.
func Load() ([]Action, error) {
	var actions []Action
	if err := json.Unmarshal(raw, &actions); err != nil {
		return nil, fmt.Errorf("parse embedded keymap: %w", err)
	}
	if err := Validate(actions); err != nil {
		return nil, err
	}
	return actions, nil
}

// Validate rejects a malformed table. A bad row must fail before anything is
// written to the desktop.
func Validate(actions []Action) error {
	seen := map[string]bool{}
	for i, a := range actions {
		if a.ID == "" {
			return fmt.Errorf("row %d: empty id", i)
		}
		if seen[a.ID] {
			return fmt.Errorf("row %d: duplicate id %q", i, a.ID)
		}
		seen[a.ID] = true
		if a.Label == "" {
			return fmt.Errorf("%s: empty label", a.ID)
		}
		if chord.Parse(a.Omarchy).Key == "" {
			return fmt.Errorf("%s: invalid omarchy chord %q", a.ID, a.Omarchy)
		}
		switch a.Kind {
		case Keep:
		case Align:
			if a.Bind == "" {
				return fmt.Errorf("%s: align needs a bind id", a.ID)
			}
		case Add:
			if a.Lua == "" {
				return fmt.Errorf("%s: add needs a lua expression", a.ID)
			}
			if strings.ContainsAny(a.Lua, "\n\r") {
				return fmt.Errorf("%s: lua expression must be one line", a.ID)
			}
		case Skip:
			if a.Reason == "" {
				return fmt.Errorf("%s: skip needs a reason", a.ID)
			}
		default:
			return fmt.Errorf("%s: unknown kind %q", a.ID, a.Kind)
		}
	}
	return nil
}

// fallbacks are the chords a displaced Ryoku bind moves to. They are curated
// where Omarchy itself uses the chord (lock, clipboard) and otherwise follow
// the "add one modifier" rule. An empty entry means: derive one generically.
var fallbacks = map[string]string{
	"shell.lock":                  "SUPER + CTRL + L", // Omarchy's own lock chord
	"shell.clipboard":             "SUPER + CTRL + V", // Omarchy's clipboard history
	"column.center":               "SUPER + ALT + C",
	"workspace.overview":          "SUPER + CTRL + Tab",
	"shell.voice":                 "SUPER + ALT + grave",
	"column.tabbed":               "SUPER + G",
	"window.float":                "SUPER + T",
	"workspace.scratchpad":        "SUPER + grave",
	"display.moveWindow.left":     "SUPER + SHIFT + CTRL + Left",
	"display.moveWindow.right":    "SUPER + SHIFT + CTRL + Right",
	"display.moveWindow.up":       "SUPER + SHIFT + CTRL + Up",
	"display.moveWindow.down":     "SUPER + SHIFT + CTRL + Down",
	"display.moveWorkspace.left":  "SUPER + SHIFT + ALT + Left",
	"display.moveWorkspace.right": "SUPER + SHIFT + ALT + Right",
	"display.moveWorkspace.up":    "SUPER + SHIFT + ALT + Up",
	"display.moveWorkspace.down":  "SUPER + SHIFT + ALT + Down",
}

// Fallback returns the curated fallback chord for a catalog bind id, or "".
func Fallback(bindID string) string { return fallbacks[bindID] }

// IsProtected reports chords that must never be claimed or moved:
// Ryoku Settings and the launcher.
func IsProtected(s string) bool {
	switch chord.Normalize(s) {
	case "SUPER+comma", "SUPER+space":
		return true
	}
	return false
}
