package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Setup commands, kept apart from the UI: `council --bind [key]` adds a conflict-free shortcut,
// `council --ensure-tab` opens a Council tab if the workspace has none, and
// `council --auto-tab on|off` makes herdr do that at every start.
//
// ponytail: dispatched from init so main's flag set stays UI-only; move into main if it grows.
func init() {
	if len(os.Args) < 2 {
		return
	}
	switch os.Args[1] {
	case "--bind":
		os.Exit(bindKey(os.Args[2:]))
	case "--ensure-tab":
		os.Exit(ensureTab(os.Args[2:]))
	case "--auto-tab":
		os.Exit(setAutoTab(os.Args[2:]))
	case "--startup":
		os.Exit(startup())
	}
}

var preferredKeys = []string{"prefix+a", "prefix+shift+a", "prefix+y", "prefix+shift+y", "prefix+comma"}

func herdrConfigPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "herdr", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "herdr", "config.toml")
}

var quoted = regexp.MustCompile(`"([^"]+)"`)

// boundKeys collects every key chord assigned in a herdr config: [keys] entries (commented-out
// defaults included, since `herdr --default-config` prints them as comments) and [[keys.command]] keys.
func boundKeys(cfg string, includeComments bool) map[string]bool {
	keys := map[string]bool{}
	section := ""
	for _, line := range strings.Split(cfg, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "#") {
			if !includeComments {
				continue
			}
			l = strings.TrimSpace(strings.TrimPrefix(l, "#"))
		}
		if strings.HasPrefix(l, "[") {
			section = strings.Trim(l, "[] ")
			continue
		}
		if !strings.HasPrefix(section, "keys") || !strings.Contains(l, "=") {
			continue
		}
		name := strings.TrimSpace(strings.SplitN(l, "=", 2)[0])
		if name == "prefix" || name == "command" || name == "type" || name == "description" {
			continue
		}
		for _, m := range quoted.FindAllStringSubmatch(strings.SplitN(l, "=", 2)[1], -1) {
			keys[strings.ToLower(m[1])] = true
		}
	}
	return keys
}

// existingBinding returns the key already bound to a council action, if any.
func existingBinding(cfg string) string {
	blocks := strings.Split(cfg, "[[keys.command]]")
	for _, b := range blocks[1:] {
		if strings.Contains(b, `"herdr-council.`) {
			if m := regexp.MustCompile(`key\s*=\s*"([^"]+)"`).FindStringSubmatch(b); m != nil {
				return m[1]
			}
		}
	}
	return ""
}

func pickKey(want string, used map[string]bool) (string, error) {
	if want != "" {
		if used[strings.ToLower(want)] {
			return "", fmt.Errorf("%s is already bound in your herdr config or herdr's defaults", want)
		}
		return want, nil
	}
	for _, k := range preferredKeys {
		if !used[k] {
			return k, nil
		}
	}
	return "", fmt.Errorf("all suggested keys are taken; pass one: council --bind prefix+<key>")
}

func bindKey(args []string) int {
	path := herdrConfigPath()
	cur, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "council:", err)
		return 1
	}
	if k := existingBinding(string(cur)); k != "" {
		fmt.Printf("Council is already on %s (%s)\n", k, path)
		return 0
	}
	defaults, err := herdr("--default-config")
	if err != nil {
		fmt.Fprintln(os.Stderr, "council: could not read herdr's default keys:", err)
		return 1
	}
	used := boundKeys(string(defaults), true)
	for k := range boundKeys(string(cur), false) {
		used[k] = true
	}
	want := ""
	if len(args) > 0 {
		want = args[0]
	}
	key, err := pickKey(want, used)
	if err != nil {
		fmt.Fprintln(os.Stderr, "council:", err)
		return 1
	}
	block := fmt.Sprintf("\n# herdr-council\n[[keys.command]]\nkey = %q\ntype = \"plugin_action\"\ncommand = \"herdr-council.open\"\ndescription = \"ask the council\"\n", key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "council:", err)
		return 1
	}
	if err := os.WriteFile(path, append(cur, []byte(block)...), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "council:", err)
		return 1
	}
	if _, err := herdr("config", "check"); err != nil { // never leave the user with a broken config
		os.WriteFile(path, cur, 0o644)
		fmt.Fprintln(os.Stderr, "council: herdr rejected the new binding, config restored:", err)
		return 1
	}
	herdr("server", "reload-config")
	fmt.Printf("Council is on %s now (%s)\n", key, path)
	return 0
}

type pane struct {
	ID        string `json:"pane_id"`
	Tab       string `json:"tab_id"`
	Workspace string `json:"workspace_id"`
	Label     string `json:"label"`
}

func councilPane(workspace string) (*pane, error) {
	out, err := herdr("pane", "list")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			Panes []pane `json:"panes"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, err
	}
	for _, p := range resp.Result.Panes {
		if p.Label == "Council" && (workspace == "" || p.Workspace == workspace) {
			return &p, nil
		}
	}
	return nil, nil
}

// ensureTab opens the Council in its own tab, named "Council", unless the workspace already has one.
func ensureTab(args []string) int {
	ws := currentWorkspace()
	if len(args) > 0 {
		ws = args[0]
	}
	if p, err := councilPane(ws); err != nil {
		fmt.Fprintln(os.Stderr, "council:", err)
		return 1
	} else if p != nil {
		herdr("tab", "rename", p.Tab, "Council")
		return 0
	}
	open := []string{"plugin", "pane", "open", "--plugin", "herdr-council", "--entrypoint", "council", "--placement", "tab"}
	if ws != "" {
		open = append(open, "--workspace", ws)
	}
	if _, err := herdr(open...); err != nil {
		fmt.Fprintln(os.Stderr, "council:", err)
		return 1
	}
	if p, err := councilPane(ws); err == nil && p != nil {
		herdr("tab", "rename", p.Tab, "Council")
	}
	return 0
}

func autoTabFile() string {
	dir := os.Getenv("HERDR_PLUGIN_CONFIG_DIR")
	if dir == "" {
		dir = filepath.Join(stateDir(), "config")
	}
	return filepath.Join(dir, "auto-tab")
}

func setAutoTab(args []string) int {
	f := autoTabFile()
	switch {
	case len(args) > 0 && args[0] == "on":
		os.MkdirAll(filepath.Dir(f), 0o755)
		if err := os.WriteFile(f, []byte("on\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "council:", err)
			return 1
		}
		fmt.Println("Council will open its own tab in every workspace when herdr starts.")
	case len(args) > 0 && args[0] == "off":
		os.Remove(f)
		fmt.Println("Council will no longer open a tab at start.")
	default:
		fmt.Fprintln(os.Stderr, "usage: council --auto-tab on|off")
		return 2
	}
	return 0
}

// startup runs from the manifest's [[startup]] hook: it does nothing unless auto-tab is on.
func startup() int {
	if _, err := os.Stat(autoTabFile()); err != nil {
		return 0
	}
	out, err := herdr("workspace", "list")
	if err != nil {
		return 0
	}
	var resp struct {
		Result struct {
			Workspaces []struct {
				ID string `json:"workspace_id"`
			} `json:"workspaces"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &resp) != nil {
		return 0
	}
	for _, w := range resp.Result.Workspaces {
		ensureTab([]string{w.ID})
	}
	return 0
}
