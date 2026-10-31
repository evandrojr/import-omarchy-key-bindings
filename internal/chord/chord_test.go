package chord

import "testing"

func TestNormalizeOrderAndCase(t *testing.T) {
	cases := map[string]string{
		"SUPER + CTRL + L": "SUPER+CTRL+l",
		"ctrl + super + l": "SUPER+CTRL+l",
		"Super+Shift+Tab":  "SUPER+SHIFT+tab",
		"Print":            "print",
		"SHIFT + Print":    "SHIFT+print",
		"SUPER + `":        "SUPER+grave",
		"ALT + Enter":      "ALT+return",
		"SUPER + comma":    "SUPER+comma",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEqual(t *testing.T) {
	if !Equal("SUPER + CTRL + L", "ctrl+super+l") {
		t.Fatal("expected equal")
	}
	if Equal("SUPER + L", "SUPER + CTRL + L") {
		t.Fatal("expected different")
	}
}

func TestDisplay(t *testing.T) {
	if got := Display("ctrl + super + left"); got != "SUPER + CTRL + left" {
		t.Fatalf("Display = %q", got)
	}
	if got := Display("Print"); got != "Print" {
		t.Fatalf("Display(Print) = %q", got)
	}
}

func TestWithMods(t *testing.T) {
	if got := WithMods("SUPER + C", "ALT"); got != "SUPER + ALT + C" {
		t.Fatalf("WithMods = %q", got)
	}
	if got := WithMods("SUPER + C", "ALT", "CTRL"); got != "SUPER + CTRL + ALT + C" {
		t.Fatalf("WithMods = %q", got)
	}
}
