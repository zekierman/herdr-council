package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Setup and lifecycle commands, kept apart from the UI:
//
//	council --bind [key]      add a conflict-free shortcut to the user's herdr config
//	council --ensure-tab      open a Council tab in this workspace unless one exists
//	council --auto-tab on|off keep (or stop keeping) a Council tab in every workspace
//	council --startup         manifest [[startup]] hook: new session, so tabs closed last time may return
//	council --event           manifest [[events]] hook: ensure on workspace/tab events, note closes
//
// ponytail: dispatched from init so main's flag set stays UI-only; move into main if it grows.
func init() {
	if len(os.Args) < 2 {
		return
	}
	switch os.Args[1] {
	case "--bind":
		os.Exit(bindCLI(os.Args[2:]))
	case "--ensure-tab":
		os.Exit(ensureTabCLI(os.Args[2:]))
	case "--auto-tab":
		os.Exit(setAutoTab(os.Args[2:]))
	case "--startup":
		os.Exit(startup())
	case "--event":
		os.Exit(onEvent())
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

// currentShortcut reports the key already bound to Council, or "".
func currentShortcut() string {
	cur, _ := os.ReadFile(herdrConfigPath())
	return existingBinding(string(cur))
}

// bindShortcut binds Council to want (or the first free suggested key) and reloads herdr.
// It returns the key in use; already is true when Council was bound before. The UI calls it too.
func bindShortcut(want string) (key string, already bool, err error) {
	path := herdrConfigPath()
	cur, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", false, err
	}
	if k := existingBinding(string(cur)); k != "" {
		return k, true, nil
	}
	defaults, err := herdr("--default-config")
	if err != nil {
		return "", false, fmt.Errorf("could not read herdr's default keys: %w", err)
	}
	used := boundKeys(string(defaults), true)
	for k := range boundKeys(string(cur), false) {
		used[k] = true
	}
	if key, err = pickKey(want, used); err != nil {
		return "", false, err
	}
	block := fmt.Sprintf("\n# herdr-council\n[[keys.command]]\nkey = %q\ntype = \"plugin_action\"\ncommand = \"herdr-council.open\"\ndescription = \"ask the council\"\n", key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(path, append(cur, []byte(block)...), 0o644); err != nil {
		return "", false, err
	}
	if _, err := herdr("config", "check"); err != nil { // never leave the user with a broken config
		os.WriteFile(path, cur, 0o644)
		return "", false, fmt.Errorf("herdr rejected the new binding, config restored: %w", err)
	}
	herdr("server", "reload-config")
	return key, false, nil
}

func bindCLI(args []string) int {
	want := ""
	if len(args) > 0 {
		want = args[0]
	}
	key, already, err := bindShortcut(want)
	switch {
	case err != nil:
		fmt.Fprintln(os.Stderr, "council:", err)
		return 1
	case already:
		fmt.Printf("Council is already on %s (%s)\n", key, herdrConfigPath())
	default:
		fmt.Printf("Council is on %s now (%s)\n", key, herdrConfigPath())
	}
	return 0
}

type pane struct {
	ID        string `json:"pane_id"`
	Tab       string `json:"tab_id"`
	Workspace string `json:"workspace_id"`
	Label     string `json:"label"`
	Agent     string `json:"agent"`
}

func listPanes() ([]pane, error) {
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
	return resp.Result.Panes, nil
}

func councilPane(workspace string) (*pane, error) {
	ps, err := listPanes()
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		if p.Label == "Council" && (workspace == "" || p.Workspace == workspace) {
			return &p, nil
		}
	}
	return nil, nil
}

// waitCouncil polls until herdr lists the Council pane: opening a tab is asynchronous.
func waitCouncil(ws string) *pane {
	for i := 0; i < 20; i++ {
		if p, err := councilPane(ws); err == nil && p != nil {
			return p
		}
		time.Sleep(150 * time.Millisecond)
	}
	return nil
}

// tidyCouncilTabs closes tabs still named "Council" after the Council pane left them, when all
// that remains is other plugins' chrome (labelled panes with no agent, e.g. a sidebar).
// A tab with an agent or a plain shell in it is never closed.
func tidyCouncilTabs(ws string) {
	ps, err := listPanes()
	if err != nil {
		return
	}
	out, err := herdr("tab", "list")
	if err != nil {
		return
	}
	var resp struct {
		Result struct {
			Tabs []struct {
				ID        string `json:"tab_id"`
				Workspace string `json:"workspace_id"`
				Label     string `json:"label"`
			} `json:"tabs"`
		} `json:"result"`
	}
	json.Unmarshal(out, &resp)
	for _, t := range resp.Result.Tabs {
		if t.Workspace != ws || t.Label != "Council" {
			continue
		}
		onlyChrome := true
		for _, p := range ps {
			if p.Tab == t.ID && (p.Label == "Council" || p.Label == "" || p.Agent != "") {
				onlyChrome = false
			}
		}
		if onlyChrome {
			herdr("tab", "close", t.ID)
		}
	}
}

func focusedTab(workspace string) string {
	out, err := herdr("tab", "list")
	if err != nil {
		return ""
	}
	var resp struct {
		Result struct {
			Tabs []struct {
				ID        string `json:"tab_id"`
				Workspace string `json:"workspace_id"`
				Focused   bool   `json:"focused"`
			} `json:"tabs"`
		} `json:"result"`
	}
	json.Unmarshal(out, &resp)
	for _, t := range resp.Result.Tabs {
		if t.Workspace == workspace && t.Focused {
			return t.ID
		}
	}
	return ""
}

// Per-workspace markers in the state dir: "known" means a Council tab was there,
// "hidden" means the user closed it this session (cleared by the next startup).
func markerPath(kind, ws string) string {
	return filepath.Join(stateDir(), kind, strings.NewReplacer(":", "_", "/", "_", `\`, "_").Replace(ws))
}
func hasMarker(kind, ws string) bool { _, err := os.Stat(markerPath(kind, ws)); return err == nil }
func setMarker(kind, ws string) {
	os.MkdirAll(filepath.Dir(markerPath(kind, ws)), 0o755)
	os.WriteFile(markerPath(kind, ws), nil, 0o644)
}
func clearMarker(kind, ws string) { os.Remove(markerPath(kind, ws)) }

// withLock serialises ensures: herdr sends focus events in bursts, and two concurrent
// ensures would each open a tab. A lock older than 30s is from a crashed run.
func withLock(fn func()) {
	dir := filepath.Join(stateDir(), "ensure.lock")
	os.MkdirAll(stateDir(), 0o755)
	for i := 0; i < 20; i++ {
		if os.Mkdir(dir, 0o755) == nil {
			defer os.Remove(dir)
			fn()
			return
		}
		if fi, err := os.Stat(dir); err == nil && time.Since(fi.ModTime()) > 30*time.Second {
			os.Remove(dir)
			continue
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// ensureTab opens a Council tab named "Council" in ws unless one exists. With keepFocus the
// user's current tab stays in front, so a background ensure never steals focus.
func ensureTab(ws string, keepFocus bool) error {
	var err error
	withLock(func() {
		var p *pane
		if p, err = councilPane(ws); err != nil {
			return
		}
		if p != nil {
			setMarker("known", ws)
			herdr("tab", "rename", p.Tab, "Council")
			return
		}
		tidyCouncilTabs(ws) // leftovers from an earlier Council pane
		back := ""
		if keepFocus {
			back = focusedTab(ws)
		}
		open := []string{"plugin", "pane", "open", "--plugin", "herdr-council", "--entrypoint", "council", "--placement", "tab"}
		if ws != "" {
			open = append(open, "--workspace", ws)
		}
		if _, err = herdr(open...); err != nil {
			return
		}
		if p = waitCouncil(ws); p != nil { // inside the lock, so a concurrent ensure sees it
			herdr("tab", "rename", p.Tab, "Council")
			setMarker("known", ws)
		}
		if back != "" && (p == nil || back != p.Tab) {
			herdr("tab", "focus", back)
		}
	})
	return err
}

func ensureTabCLI(args []string) int {
	ws := currentWorkspace()
	if len(args) > 0 {
		ws = args[0]
	}
	clearMarker("hidden", ws) // an explicit request brings it back even after a close
	if err := ensureTab(ws, false); err != nil {
		fmt.Fprintln(os.Stderr, "council:", err)
		return 1
	}
	return 0
}

func setAutoTab(args []string) int {
	s := loadSettings()
	switch {
	case len(args) > 0 && args[0] == "on":
		s.AutoTab = true
	case len(args) > 0 && args[0] == "off":
		s.AutoTab = false
	default:
		fmt.Fprintln(os.Stderr, "usage: council --auto-tab on|off")
		return 2
	}
	if err := saveSettings(s); err != nil {
		fmt.Fprintln(os.Stderr, "council:", err)
		return 1
	}
	if s.AutoTab {
		fmt.Println("Council keeps a tab in every workspace.")
	} else {
		fmt.Println("Council no longer opens tabs on its own.")
	}
	return 0
}

func workspaces() []string {
	out, err := herdr("workspace", "list")
	if err != nil {
		return nil
	}
	var resp struct {
		Result struct {
			Workspaces []struct {
				ID string `json:"workspace_id"`
			} `json:"workspaces"`
		} `json:"result"`
	}
	json.Unmarshal(out, &resp)
	var ids []string
	for _, w := range resp.Result.Workspaces {
		ids = append(ids, w.ID)
	}
	return ids
}

// startup: a new herdr session, so tabs the user closed last session may come back.
func startup() int {
	os.RemoveAll(filepath.Join(stateDir(), "hidden"))
	if !loadSettings().AutoTab {
		return 0
	}
	for _, ws := range workspaces() {
		ensureTab(ws, true)
	}
	return 0
}

// eventWorkspace finds the workspace an event is about: event JSON first, then the context.
func eventWorkspace() string {
	for _, env := range []string{"HERDR_PLUGIN_EVENT_JSON", "HERDR_PLUGIN_CONTEXT_JSON"} {
		if m := regexp.MustCompile(`"workspace_id"\s*:\s*"([^"]+)"`).FindStringSubmatch(os.Getenv(env)); m != nil {
			return m[1]
		}
	}
	return os.Getenv("HERDR_WORKSPACE_ID")
}

// onEvent: pane.closed notes a closed Council tab (hidden until the next session); every other
// event makes sure the workspace has its Council tab, unless the user turned that off or closed it.
func onEvent() int {
	ws := eventWorkspace()
	if ws == "" {
		return 0
	}
	if os.Getenv("HERDR_PLUGIN_EVENT") == "pane.closed" {
		if hasMarker("known", ws) {
			if p, err := councilPane(ws); err == nil && p == nil {
				setMarker("hidden", ws)
				clearMarker("known", ws)
				tidyCouncilTabs(ws)
			}
		}
		return 0
	}
	if !loadSettings().AutoTab || hasMarker("hidden", ws) {
		return 0
	}
	ensureTab(ws, true)
	return 0
}
