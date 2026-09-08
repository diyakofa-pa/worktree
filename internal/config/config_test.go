package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWritesDefaultsOnFirstRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path failed: %v", err)
	}
	if filepath.Base(filepath.Dir(path)) != "worktree" {
		t.Errorf("expected the configuration under a worktree directory, got %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected the defaults to be written to disk: %v", err)
	}

	enabled := cfg.Enabled()
	if len(enabled) != 2 || enabled[0].Key != "cursor" || enabled[1].Key != "vscode" {
		t.Errorf("expected cursor and vscode enabled in that order, got %+v", enabled)
	}

	// The file that was just written must load back unchanged.
	reloaded, err := Load()
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if len(reloaded.All()) != len(cfg.All()) {
		t.Errorf("expected the configuration to round trip, got %+v", reloaded.All())
	}
}

func TestFindResolvesEditorsAndDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := Default()

	editor, err := cfg.Find("")
	if err != nil || editor.Key != "cursor" {
		t.Errorf("expected the first enabled editor by default, got %+v (%v)", editor, err)
	}

	for _, name := range []string{"vscode", "VS Code", "code"} {
		if editor, err := cfg.Find(name); err != nil || editor.Key != "vscode" {
			t.Errorf("Find(%q) = %+v, %v", name, editor, err)
		}
	}

	// A disabled editor is still resolvable when it is named explicitly.
	if editor, err := cfg.Find("zed"); err != nil || editor.Command != "zed" {
		t.Errorf("expected a disabled editor to be resolvable, got %+v (%v)", editor, err)
	}

	if _, err := cfg.Find("emacs"); err == nil {
		t.Error("expected an unknown editor to be rejected")
	}

	empty := &Config{Editors: map[string]Editor{"zed": {Command: "zed"}}}
	if _, err := empty.Find(""); err == nil {
		t.Error("expected an error when no editor is enabled")
	}
}

func TestCorruptConfigFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "worktree", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create the config directory: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("failed to write the config: %v", err)
	}

	cfg, err := Load()
	if err == nil {
		t.Error("expected a parse error to be reported")
	}
	if len(cfg.Enabled()) != 2 {
		t.Errorf("expected the defaults to be used, got %+v", cfg.Enabled())
	}
}
