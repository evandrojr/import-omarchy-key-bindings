// Command import-omarchy-key-bindings ports Omarchy's hotkeys onto a running
// Ryoku (Hyprland) desktop. It plans first, writes only rebinds.lua's store and
// the managed block in user.lua, and rolls back automatically when the reload
// or the Lua parse fails.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/evandrojr/import-omarchy-key-bindings/internal/backup"
	"github.com/evandrojr/import-omarchy-key-bindings/internal/chord"
	"github.com/evandrojr/import-omarchy-key-bindings/internal/keymap"
	"github.com/evandrojr/import-omarchy-key-bindings/internal/omarchy"
	"github.com/evandrojr/import-omarchy-key-bindings/internal/planner"
	"github.com/evandrojr/import-omarchy-key-bindings/internal/render"
	"github.com/evandrojr/import-omarchy-key-bindings/internal/ryoku"
	"github.com/evandrojr/import-omarchy-key-bindings/internal/userlua"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "plan":
		err = cmdPlan(os.Args[2:])
	case "apply":
		err = cmdApply(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "restore":
		err = cmdRestore(os.Args[2:])
	case "mappings":
		err = cmdMappings(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("import-omarchy-key-bindings " + version)
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nERROR: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`import-omarchy-key-bindings ` + version + `

Ports Omarchy's hotkeys onto a running Ryoku (Hyprland) desktop.

  plan     [--json] [--adopt] [--omarchy-dir DIR]   show the change set, write nothing
  apply    [--yes] [--adopt] [--no-reload]          back up, rebind, write the block, verify
  verify                                            parse + configerrors + live chord check
  restore  [--backup DIR]                           put the latest backup back
  mappings                                          print the curated table
  version

Common flags:
  --user-lua PATH     default ~/.config/hypr/user.lua
  --state-dir PATH    default ~/.local/state/import-omarchy-key-bindings
  --adopt             take over hand-written binds that sit on a mapped chord
  --omarchy-dir DIR   read Omarchy's bindings/*.lua and follow its current chords
  --no-reload         do not ask the compositor to reload
`)
}

type options struct {
	userLua    string
	stateDir   string
	omarchyDir string
	adopt      bool
	json       bool
	yes        bool
	noReload   bool
	backupDir  string
}

func defaultUserLua() string { p, _ := ryoku.ConfigPath("hypr", "user.lua"); return p }
func defaultStateDir() string {
	p, _ := ryoku.StatePath("import-omarchy-key-bindings")
	return p
}

func addCommon(fs *flag.FlagSet, o *options) {
	fs.StringVar(&o.userLua, "user-lua", defaultUserLua(), "path to user.lua")
	fs.StringVar(&o.stateDir, "state-dir", defaultStateDir(), "backup state directory")
}

type app struct {
	o       options
	client  ryoku.Client
	actions []keymap.Action
	catalog ryoku.Catalog
	plan    planner.Plan
	src     string
	notes   []string
}

func prepare(o options) (*app, error) {
	a := &app{o: o, client: ryoku.New()}
	if err := a.client.EnsureHyprland(); err != nil {
		return nil, err
	}
	actions, err := keymap.Load()
	if err != nil {
		return nil, err
	}
	if o.omarchyDir != "" {
		notes, err := applyOmarchyDir(actions, o.omarchyDir)
		if err != nil {
			return nil, err
		}
		a.notes = notes
	}
	cat, err := a.client.Keybinds()
	if err != nil {
		return nil, err
	}
	rebinds, err := a.client.Rebinds()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(o.userLua)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	a.src = string(raw)
	managed, manual := userlua.Bindings(a.src)
	a.actions = actions
	a.catalog = cat
	a.plan, err = planner.Build(planner.Input{
		Actions: actions,
		Catalog: cat,
		Rebinds: rebinds,
		Managed: managed,
		Manual:  manual,
		Adopt:   o.adopt,
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

func applyOmarchyDir(actions []keymap.Action, dir string) ([]string, error) {
	d, err := omarchy.Dir(dir)
	if err != nil {
		return nil, err
	}
	var notes []string
	for i := range actions {
		if actions[i].OmarchyLabel == "" {
			continue
		}
		c, ok := d.ChordFor(actions[i].OmarchyLabel)
		if !ok {
			notes = append(notes, fmt.Sprintf("%s: %q not found in %s; keeping %s", actions[i].ID, actions[i].OmarchyLabel, dir, actions[i].Omarchy))
			continue
		}
		if !chord.Equal(c, actions[i].Omarchy) {
			notes = append(notes, fmt.Sprintf("%s: %s -> %s (from %s)", actions[i].ID, actions[i].Omarchy, chord.Display(c), dir))
			actions[i].Omarchy = chord.Display(c)
		}
	}
	return notes, nil
}

func (a *app) errorRows() []planner.Row {
	var out []planner.Row
	for _, r := range a.plan.Rows {
		if r.Result == planner.ResultError {
			out = append(out, r)
		}
	}
	return out
}

func (a *app) printPlan() {
	fmt.Printf("import-omarchy-key-bindings %s  (user.lua: %s)\n\n", version, a.o.userLua)
	for _, n := range a.notes {
		fmt.Println("omarchy:", n)
	}
	if len(a.notes) > 0 {
		fmt.Println()
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ACTION\tOMARCHY\tRYOKU NOW\tRESULT\tDETAIL")
	for _, r := range a.plan.Rows {
		detail := r.Detail
		if r.Result == planner.ResultSkipped {
			detail = r.Detail
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Action.Label, chord.Display(r.Action.Omarchy), r.Now, r.Result, detail)
	}
	w.Flush()

	counts := map[string]int{}
	for _, r := range a.plan.Rows {
		counts[r.Result]++
	}
	fmt.Printf("\n%d rows: %d same, %d moved, %d new, %d conflict resolved, %d not ported, %d error\n",
		len(a.plan.Rows), counts[planner.ResultSame], counts[planner.ResultMoved], counts[planner.ResultNew],
		counts[planner.ResultConflict], counts[planner.ResultSkipped], counts[planner.ResultError])
	if len(a.plan.Moves) > 0 {
		fmt.Println("\nRebinds (store):")
		for _, m := range a.plan.Moves {
			fmt.Printf("  %-34s %s -> %s\n", m.Label, chord.Display(m.From), chord.Display(m.To))
		}
	}
	if len(a.plan.Adds) > 0 {
		fmt.Println("\nNew binds (user.lua block):")
		for _, ad := range a.plan.Adds {
			fmt.Printf("  %-34s %s\n", ad.Label, ad.Chord)
		}
	}
	if len(a.plan.Adopted) > 0 {
		fmt.Println("\nAdopted from the hand-written part of user.lua (will be removed):")
		for _, c := range a.plan.Adopted {
			fmt.Printf("  %s\n", c)
		}
	}
	for _, w := range a.plan.Warnings {
		fmt.Println("warning:", w)
	}
}

func cmdPlan(args []string) error {
	var o options
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	addCommon(fs, &o)
	fs.BoolVar(&o.adopt, "adopt", false, "take over hand-written binds on mapped chords")
	fs.StringVar(&o.omarchyDir, "omarchy-dir", "", "read Omarchy bindings from this checkout")
	fs.BoolVar(&o.json, "json", false, "print JSON")
	_ = fs.Parse(args)

	a, err := prepare(o)
	if err != nil {
		return err
	}
	if o.json {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(a.plan)
	}
	a.printPlan()
	return nil
}

func cmdApply(args []string) error {
	var o options
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	addCommon(fs, &o)
	fs.BoolVar(&o.adopt, "adopt", false, "take over hand-written binds on mapped chords")
	fs.StringVar(&o.omarchyDir, "omarchy-dir", "", "read Omarchy bindings from this checkout")
	fs.BoolVar(&o.yes, "yes", false, "do not ask for confirmation")
	fs.BoolVar(&o.noReload, "no-reload", false, "skip the compositor reload")
	_ = fs.Parse(args)

	a, err := prepare(o)
	if err != nil {
		return err
	}
	a.printPlan()
	if rows := a.errorRows(); len(rows) > 0 {
		return fmt.Errorf("%d row(s) need attention; nothing was written", len(rows))
	}
	if a.nothingToDo() {
		fmt.Println("\nNothing to do: the desktop already matches this plan.")
		return nil
	}
	if !o.yes && !confirm() {
		return fmt.Errorf("aborted")
	}
	return a.apply()
}

func (a *app) nothingToDo() bool {
	if len(a.plan.Moves) > 0 || len(a.plan.Adopted) > 0 {
		return false
	}
	_, block, _, found := userlua.Split(a.src)
	return found && strings.TrimSpace(block) == strings.TrimSpace(render.Block(a.plan.Adds))
}

func confirm() bool {
	fmt.Print("\nApply these changes? [y/N] ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

func (a *app) paths() (userLua, rebindsLua, desktopJSON string) {
	userLua = a.o.userLua
	rebindsLua, _ = ryoku.ConfigPath("hypr", "rebinds.lua")
	desktopJSON, _ = ryoku.ConfigPath("ryoku", "desktop.json")
	return
}

func (a *app) apply() error {
	userLua, rebindsLua, desktopJSON := a.paths()
	var entries []backup.Entry
	if _, err := os.Stat(userLua); err == nil {
		entries = append(entries, backup.Entry{Source: userLua, Rel: "hypr/user.lua"})
	}
	if _, err := os.Stat(rebindsLua); err == nil {
		entries = append(entries, backup.Entry{Source: rebindsLua, Rel: "hypr/rebinds.lua"})
	}
	if _, err := os.Stat(desktopJSON); err == nil {
		entries = append(entries, backup.Entry{Source: desktopJSON, Rel: "ryoku/desktop.json"})
	}
	meta := map[string]any{"version": version, "moves": a.plan.Moves, "adds": a.plan.Adds, "adopted": a.plan.Adopted}
	bk, err := backup.Create(a.o.stateDir, entries, meta)
	if err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	fmt.Printf("\nBackup: %s\n", bk.Dir)

	rollback := func(cause error) error {
		if rerr := bk.Restore(); rerr != nil {
			return fmt.Errorf("%w; ROLLBACK ALSO FAILED: %v (backup at %s)", cause, rerr, bk.Dir)
		}
		_ = a.client.Reload()
		return fmt.Errorf("%w; rolled back from %s", cause, bk.Dir)
	}

	for _, m := range a.plan.Moves {
		fmt.Printf("rebind %-30s %s -> %s\n", m.Label, chord.Display(m.Default), chord.Display(m.To))
		if err := a.client.SetRebind(m.Default, m.To); err != nil {
			return rollback(err)
		}
	}

	src, err := os.ReadFile(userLua)
	if err != nil && !os.IsNotExist(err) {
		return rollback(err)
	}
	text := string(src)
	if len(a.plan.Adopted) > 0 {
		set := map[string]bool{}
		for _, c := range a.plan.Adopted {
			set[chord.Normalize(c)] = true
		}
		var removed []string
		text, removed = userlua.RemoveBinds(text, set)
		if len(removed) > 0 {
			fmt.Println("adopted:", strings.Join(removed, ", "))
		}
	}
	text = userlua.Upsert(text, render.Block(a.plan.Adds))
	if err := os.MkdirAll(filepath.Dir(userLua), 0o755); err != nil {
		return rollback(err)
	}
	if err := os.WriteFile(userLua, []byte(text), 0o644); err != nil {
		return rollback(err)
	}

	if err := a.verify(userLua, !a.o.noReload, !a.o.noReload); err != nil {
		return rollback(err)
	}
	fmt.Printf("\nApplied. Backup: %s\n", bk.Dir)
	fmt.Println("Revert with: import-omarchy-key-bindings restore")
	return nil
}

func (a *app) verify(userLua string, reload, checkLive bool) error {
	if checked, err := a.client.CheckLua(userLua); err != nil {
		return fmt.Errorf("lua parse failed: %w", err)
	} else if !checked {
		fmt.Println("warning: luac not found, skipping the Lua parse")
	}
	if reload {
		if err := a.client.Reload(); err != nil {
			return fmt.Errorf("reload: %w", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !checkLive {
		fmt.Println("verify: reload skipped, live chord check skipped")
		return nil
	}
	errs, err := a.client.ConfigErrors()
	if err != nil {
		return err
	}
	if errs != "" {
		return fmt.Errorf("compositor reported config errors:\n%s", errs)
	}
	live, err := a.client.LiveBinds()
	if err != nil {
		return err
	}
	count := map[string]int{}
	for _, b := range live {
		count[chord.Normalize(b.Chord())]++
	}
	var missing, duplicated []string
	for want, label := range a.plan.Targets {
		switch {
		case count[want] == 0:
			missing = append(missing, fmt.Sprintf("%s (%s)", label, want))
		case count[want] > 1:
			duplicated = append(duplicated, fmt.Sprintf("%s (%s)", label, want))
		}
	}
	sort.Strings(missing)
	sort.Strings(duplicated)
	if len(missing) > 0 {
		return fmt.Errorf("chords not live after reload: %s", strings.Join(missing, "; "))
	}
	if len(duplicated) > 0 {
		return fmt.Errorf("chords registered twice after reload: %s", strings.Join(duplicated, "; "))
	}
	fmt.Printf("verify: %d live binds, %d target chords present, configerrors empty\n", len(live), len(a.plan.Targets))
	return nil
}

func cmdVerify(args []string) error {
	var o options
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	addCommon(fs, &o)
	_ = fs.Parse(args)
	a, err := prepare(o)
	if err != nil {
		return err
	}
	return a.verify(o.userLua, false, true)
}

func cmdRestore(args []string) error {
	var o options
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	addCommon(fs, &o)
	fs.StringVar(&o.backupDir, "backup", "", "backup directory (default: latest)")
	fs.BoolVar(&o.noReload, "no-reload", false, "skip the compositor reload")
	_ = fs.Parse(args)

	var bk backup.Backup
	var err error
	if o.backupDir != "" {
		bk, err = backup.Open(o.backupDir)
	} else {
		bk, err = backup.Latest(o.stateDir)
	}
	if err != nil {
		return err
	}
	if err := bk.Restore(); err != nil {
		return err
	}
	for _, e := range bk.Manifest.Entries {
		fmt.Printf("restored %s\n", e.Source)
	}
	client := ryoku.New()
	if !o.noReload {
		if err := client.Reload(); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
	}
	if errs, err := client.ConfigErrors(); err == nil && errs != "" {
		return fmt.Errorf("restored, but the compositor reports config errors:\n%s", errs)
	}
	fmt.Printf("restored from %s\n", bk.Dir)
	return nil
}

func cmdMappings(args []string) error {
	var o options
	fs := flag.NewFlagSet("mappings", flag.ExitOnError)
	fs.BoolVar(&o.json, "json", false, "print JSON")
	_ = fs.Parse(args)
	actions, err := keymap.Load()
	if err != nil {
		return err
	}
	if o.json {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(actions)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tKIND\tOMARCHY\tTARGET")
	for _, a := range actions {
		target := a.Bind
		if target == "" {
			target = a.Lua
		}
		if a.Kind == keymap.Skip {
			target = a.Reason
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.ID, a.Kind, a.Omarchy, target)
	}
	return w.Flush()
}
