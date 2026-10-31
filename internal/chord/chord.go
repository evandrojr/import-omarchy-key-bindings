// Package chord parses and normalises Hyprland-style key chords such as
// "SUPER + CTRL + Left". It is deliberately dependency-free so the planner can
// compare chords cheaply and deterministically.
package chord

import (
	"sort"
	"strings"
)

// canonical modifier order used for display and comparison.
var modOrder = []string{"SUPER", "CTRL", "ALT", "SHIFT"}

var modAliases = map[string]string{
	"super": "SUPER", "mod": "SUPER", "meta": "SUPER", "win": "SUPER", "logo": "SUPER",
	"ctrl": "CTRL", "control": "CTRL",
	"alt": "ALT", "option": "ALT",
	"shift": "SHIFT",
}

var keyAliases = map[string]string{
	"enter":    "return",
	"return":   "return",
	"esc":      "escape",
	"escape":   "escape",
	"`":        "grave",
	"grave":    "grave",
	",":        "comma",
	"comma":    "comma",
	"pageup":   "prior",
	"prior":    "prior",
	"pagedown": "next",
	"next":     "next",
	" ":        "space",
	"space":    "space",
}

// Chord is a parsed key combination.
type Chord struct {
	Mods []string
	Key  string
}

// Parse splits a chord on "+" and classifies the tokens. A modifier token is
// only a modifier when it is not the final token (so a hypothetical key named
// "alt" still works as a key).
func Parse(s string) Chord {
	parts := strings.Split(s, "+")
	c := Chord{}
	for i, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		if m, ok := modAliases[strings.ToLower(t)]; ok && i < len(parts)-1 {
			c.Mods = append(c.Mods, m)
			continue
		}
		c.Key = t
	}
	c.Mods = orderMods(c.Mods)
	return c
}

func orderMods(mods []string) []string {
	seen := map[string]bool{}
	for _, m := range mods {
		seen[m] = true
	}
	out := make([]string, 0, len(seen))
	for _, m := range modOrder {
		if seen[m] {
			out = append(out, m)
		}
	}
	return out
}

// Normalize returns a stable key for comparisons: modifiers in canonical order,
// lower-case key with common aliases folded together. Display strings taken
// from different sources ("SUPER + Left", "super+left") compare equal.
func Normalize(s string) string {
	c := Parse(s)
	key := strings.ToLower(c.Key)
	if alias, ok := keyAliases[key]; ok {
		key = alias
	}
	return strings.Join(append(c.Mods, key), "+")
}

// Equal reports whether two chords are the same combination.
func Equal(a, b string) bool { return Normalize(a) == Normalize(b) }

// Display renders a chord with canonical modifier order and " + " separators.
func Display(s string) string {
	c := Parse(s)
	if len(c.Mods) == 0 {
		return c.Key
	}
	return strings.Join(append(c.Mods, c.Key), " + ")
}

// WithMods returns s with extra modifiers added, in canonical order.
func WithMods(s string, mods ...string) string {
	c := Parse(s)
	for _, m := range mods {
		if canonical, ok := modAliases[strings.ToLower(m)]; ok {
			c.Mods = append(c.Mods, canonical)
		}
	}
	sort.Strings(c.Mods)
	c.Mods = orderMods(c.Mods)
	return Display(strings.Join(append(c.Mods, c.Key), " + "))
}
