// Package ryoku is the thin boundary to the running Ryoku desktop: it reads the
// bind catalog and the rebind store, writes rebinds through the owning command,
// and verifies the live compositor. Nothing here edits a shipped file.
package ryoku

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/evandrojr/import-omarchy-key-bindings/internal/chord"
)

// Bind is one row of `ryoku-hub keybinds`.
type Bind struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Combo      string `json:"combo"`
	Default    string `json:"default"`
	Rebindable bool   `json:"rebindable"`
	Locked     bool   `json:"locked"`
	Unhonored  string `json:"unhonored"`
}

// Chord returns the shipped chord for the bind.
func (b Bind) Chord() string {
	if b.Combo != "" {
		return b.Combo
	}
	return b.Default
}

// Catalog is the parsed `ryoku-hub keybinds` document.
type Catalog struct {
	Categories []struct {
		Name  string `json:"name"`
		Binds []Bind `json:"binds"`
	} `json:"categories"`
}

// All flattens the catalog.
func (c Catalog) All() []Bind {
	var out []Bind
	for _, cat := range c.Categories {
		out = append(out, cat.Binds...)
	}
	return out
}

// ByID finds a bind by its stable catalog id.
func (c Catalog) ByID(id string) (Bind, bool) {
	for _, b := range c.All() {
		if b.ID == id {
			return b, true
		}
	}
	return Bind{}, false
}

// Client shells out to the Ryoku tools.
type Client struct {
	Hub     string
	Ryoku   string
	Hyprctl string
}

// New returns a Client that uses the tools on PATH.
func New() Client {
	return Client{Hub: "ryoku-hub", Ryoku: "ryoku", Hyprctl: "hyprctl"}
}

func (c Client) run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		return out.String(), fmt.Errorf("%s %s: %w (%s)", name, strings.Join(args, " "), err, msg)
	}
	return out.String(), nil
}

// EnsureHyprland refuses to run against niri or a dead session.
func (c Client) EnsureHyprland() error {
	out, err := c.run(c.Ryoku, "wm", "status")
	if err != nil {
		return err
	}
	if !strings.Contains(out, "Provider: hyprland") {
		return fmt.Errorf("active window manager is not Hyprland:\n%s", strings.TrimSpace(out))
	}
	if !strings.Contains(out, "Live: true") {
		return fmt.Errorf("the Hyprland session is not live:\n%s", strings.TrimSpace(out))
	}
	return nil
}

// Keybinds returns the shipped bind catalog with the current legend.
func (c Client) Keybinds() (Catalog, error) {
	out, err := c.run(c.Hub, "keybinds")
	if err != nil {
		return Catalog{}, err
	}
	var cat Catalog
	if err := json.Unmarshal([]byte(out), &cat); err != nil {
		return Catalog{}, fmt.Errorf("parse `ryoku-hub keybinds`: %w", err)
	}
	return cat, nil
}

// Rebinds returns the current default-chord -> chosen-chord map from the store.
func (c Client) Rebinds() (map[string]string, error) {
	out, err := c.run(c.Hub, "desktop", "get")
	if err != nil {
		return nil, err
	}
	var doc struct {
		Desktop struct {
			Rebinds map[string]string `json:"keybindRebinds"`
		} `json:"desktop"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		return nil, fmt.Errorf("parse `ryoku-hub desktop get`: %w", err)
	}
	if doc.Desktop.Rebinds == nil {
		return map[string]string{}, nil
	}
	return doc.Desktop.Rebinds, nil
}

// SetRebind records one replacement chord. It is the same writer the Keybinds
// page uses, so the GUI and this tool never disagree.
func (c Client) SetRebind(from, to string) error {
	_, err := c.run(c.Hub, "desktop", "set-rebind", from, to)
	return err
}

// Reload re-runs the Hyprland config.
func (c Client) Reload() error {
	_, err := c.run(c.Ryoku, "wm", "act", "config.reload")
	return err
}

// ConfigErrors returns the compositor's config-error overlay text (empty = ok).
func (c Client) ConfigErrors() (string, error) {
	out, err := c.run(c.Hyprctl, "configerrors")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// LiveBind is one running bind from `hyprctl binds -j`.
type LiveBind struct {
	Modmask    int    `json:"modmask"`
	Key        string `json:"key"`
	Dispatcher string `json:"dispatcher"`
}

// Chord renders the bind as a chord string.
func (l LiveBind) Chord() string {
	var mods []string
	if l.Modmask&64 != 0 {
		mods = append(mods, "SUPER")
	}
	if l.Modmask&4 != 0 {
		mods = append(mods, "CTRL")
	}
	if l.Modmask&8 != 0 {
		mods = append(mods, "ALT")
	}
	if l.Modmask&1 != 0 {
		mods = append(mods, "SHIFT")
	}
	return chord.Display(strings.Join(append(mods, l.Key), " + "))
}

// LiveBinds returns every bind the compositor currently has registered.
func (c Client) LiveBinds() ([]LiveBind, error) {
	out, err := c.run(c.Hyprctl, "binds", "-j")
	if err != nil {
		return nil, err
	}
	var binds []LiveBind
	if err := json.Unmarshal([]byte(out), &binds); err != nil {
		return nil, fmt.Errorf("parse `hyprctl binds -j`: %w", err)
	}
	return binds, nil
}

// CheckLua parses a Lua file when luac is available. checked=false means the
// tool is absent, which is not an error.
func (c Client) CheckLua(path string) (checked bool, err error) {
	luac, lookErr := exec.LookPath("luac")
	if lookErr != nil {
		return false, nil
	}
	_, err = c.run(luac, "-p", path)
	return true, err
}

// Home returns the user's config directory, honouring XDG_CONFIG_HOME.
func Home() (string, error) {
	if h := os.Getenv("HOME"); h != "" {
		return h, nil
	}
	return os.UserHomeDir()
}

// ConfigPath joins a path under ~/.config.
func ConfigPath(parts ...string) (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(append([]string{base}, parts...)...), nil
}

// StatePath joins a path under ~/.local/state.
func StatePath(parts ...string) (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(append([]string{base}, parts...)...), nil
}
