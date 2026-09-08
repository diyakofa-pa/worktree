// Package config reads the user level worktree configuration, which decides
// which editors are offered by the "open" command and by the TUI.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Editor is a single configurable editor entry.
type Editor struct {
	Enabled     bool   `json:"enabled"`
	Command     string `json:"command"`
	DisplayName string `json:"displayName"`
}

// Config is the on disk configuration document.
type Config struct {
	Editors map[string]Editor `json:"editors"`
}

// NamedEditor is an editor together with the key it is configured under.
type NamedEditor struct {
	Key string `json:"key"`
	Editor
}

// editorOrder keeps the editor listing stable, since the configuration stores
// editors in a map.
var editorOrder = []string{"cursor", "vscode", "zed", "sublime", "neovim"}

// Default is the configuration written on first run.
func Default() *Config {
	return &Config{Editors: map[string]Editor{
		"cursor":  {Enabled: true, Command: "cursor", DisplayName: "Cursor"},
		"vscode":  {Enabled: true, Command: "code", DisplayName: "VS Code"},
		"zed":     {Enabled: false, Command: "zed", DisplayName: "Zed"},
		"sublime": {Enabled: false, Command: "subl", DisplayName: "Sublime Text"},
		"neovim":  {Enabled: false, Command: "nvim", DisplayName: "Neovim"},
	}}
}

// Path is the location of the configuration file.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve the user config directory: %w", err)
	}

	return filepath.Join(dir, "worktree", "config.json"), nil
}

// Load reads the configuration, creating it with defaults on first run. A
// configuration that cannot be written is not fatal: the defaults are used.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return Default(), err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := Default()
		return cfg, cfg.Save()
	}
	if err != nil {
		return Default(), fmt.Errorf("failed to read %q: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("failed to parse %q: %w", path, err)
	}
	if len(cfg.Editors) == 0 {
		cfg.Editors = Default().Editors
	}

	return &cfg, nil
}

// Save writes the configuration back to disk.
func (c *Config) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create %q: %w", filepath.Dir(path), err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode the configuration: %w", err)
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Enabled returns the enabled editors in a stable order.
func (c *Config) Enabled() []NamedEditor {
	var enabled []NamedEditor
	for _, editor := range c.All() {
		if editor.Enabled && editor.Command != "" {
			enabled = append(enabled, editor)
		}
	}

	return enabled
}

// All returns every configured editor, known ones first in their canonical
// order and any user added ones after, sorted by key.
func (c *Config) All() []NamedEditor {
	var editors []NamedEditor
	seen := map[string]bool{}

	for _, key := range editorOrder {
		if editor, ok := c.Editors[key]; ok {
			editors = append(editors, NamedEditor{Key: key, Editor: editor})
			seen[key] = true
		}
	}

	var extra []NamedEditor
	for key, editor := range c.Editors {
		if !seen[key] {
			extra = append(extra, NamedEditor{Key: key, Editor: editor})
		}
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i].Key < extra[j].Key })

	return append(editors, extra...)
}

// Find looks an editor up by its key, its display name, or its command,
// ignoring case. Disabled editors are still resolvable so that naming one
// explicitly works without editing the configuration first.
func (c *Config) Find(name string) (NamedEditor, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		enabled := c.Enabled()
		if len(enabled) == 0 {
			path, _ := Path()
			return NamedEditor{}, fmt.Errorf("no editor is enabled in %s", path)
		}
		return enabled[0], nil
	}

	for _, editor := range c.All() {
		if strings.EqualFold(editor.Key, name) ||
			strings.EqualFold(editor.DisplayName, name) ||
			strings.EqualFold(editor.Command, name) {
			return editor, nil
		}
	}

	return NamedEditor{}, fmt.Errorf("unknown editor %q: configured editors are %s", name, strings.Join(c.keys(), ", "))
}

func (c *Config) keys() []string {
	var keys []string
	for _, editor := range c.All() {
		keys = append(keys, editor.Key)
	}

	return keys
}
