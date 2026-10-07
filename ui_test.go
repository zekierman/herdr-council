package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var styleEscape = regexp.MustCompile(`\x1b\[[0-9;:]*[A-Za-z]`)

func uiAgents() []Agent {
	return []Agent{
		{Name: "agy", Pane: "p1", Status: "idle"},
		{Name: "claude", Pane: "p2", Status: "idle"},
		{Name: "codex", Pane: "p3", Status: "idle"},
	}
}

func askingModel(w, h int) model {
	m := newModel("w1")
	m.w, m.h = w, h
	m.agents = uiAgents()
	m.judge = 1
	m.picked = map[string]bool{"p1": true, "p2": false, "p3": true}
	m.resize()
	m.input.SetValue("Where should we start?")
	return m
}

func watchingModel(w, h int) model {
	m := askingModel(w, h)
	now := time.Now()
	m.phase = watching
	m.page = "run"
	m.run = &Run{
		Question: "Where should we start?",
		Seats: []*Seat{
			{Agent: m.agents[0], State: Done, Sent: now.Add(-32 * time.Second), Finished: now.Add(-5 * time.Second), Answer: "Start with a modular monolith."},
			{Agent: m.agents[2], State: Working, Sent: now.Add(-16 * time.Second)},
		},
		Judge: &Seat{Agent: m.agents[1], State: Done, Sent: now.Add(-12 * time.Second), Finished: now.Add(-2 * time.Second), Answer: "## Consensus\nStart small.\n\n## Final answer\nUse clear modules."},
		Order: []int{0, 1},
	}
	m.seat = len(m.run.Seats)
	m.refreshAnswer()
	return m
}

func assertScreen(t *testing.T, m model) {
	t.Helper()
	sc := m.layout()
	if len(sc.lines) > m.h {
		t.Fatalf("%d lines exceed height %d", len(sc.lines), m.h)
	}
	for y, line := range sc.lines {
		if got := lipgloss.Width(line); got > m.w {
			t.Errorf("row %d width %d exceeds %d: %q", y, got, m.w, styleEscape.ReplaceAllString(line, ""))
		}
	}
	for _, z := range sc.zones {
		if z.y < 0 || z.y >= len(sc.lines) || z.x0 < 0 || z.x1 > lipgloss.Width(sc.lines[z.y]) || z.x1 <= z.x0 {
			t.Errorf("zone %+v does not match a visible span", z)
		}
	}
}

func TestUILayoutFits(t *testing.T) {
	for _, size := range [][2]int{{100, 32}, {88, 28}, {60, 24}} {
		for _, m := range []model{askingModel(size[0], size[1]), watchingModel(size[0], size[1])} {
			assertScreen(t, m)
		}
	}
	wide := styleEscape.ReplaceAllString(strings.Join(askingModel(100, 32).layout().lines, "\n"), "")
	narrow := styleEscape.ReplaceAllString(strings.Join(askingModel(60, 24).layout().lines, "\n"), "")
	if strings.Contains(wide, strings.TrimSpace(councilArt[3])) || strings.Contains(narrow, strings.TrimSpace(councilArt[3])) {
		t.Fatal("art should stay on the welcome screen")
	}
}

func TestCouncilArtSymmetry(t *testing.T) {
	if len(councilArt) != artHeight {
		t.Fatalf("art height %d", len(councilArt))
	}
	for y, row := range councilArt {
		if n := len([]rune(row)); n != artWidth || lipgloss.Width(row) != artWidth {
			t.Fatalf("row %d: %d runes, %d cells", y, n, lipgloss.Width(row))
		}
	}
	for y := range artDots {
		for x := range artDots[y] {
			if artDots[y][x] != artDots[y][dotsW-1-x] {
				t.Fatalf("dot (%d,%d) differs from its mirror", x, y)
			}
		}
	}
}

func pressKey(m model, code rune, mod tea.KeyMod) (model, tea.Cmd) {
	updated, cmd := m.Update(tea.KeyPressMsg{Code: code, Mod: mod})
	return updated.(model), cmd
}

func TestAskFocusAndKeys(t *testing.T) {
	fakeAgents(t)
	m := askingModel(100, 32)
	for _, want := range []askFocus{focusSeats, focusJudge, focusPeer, focusAsk, focusClose, focusQuestion} {
		m, _ = pressKey(m, tea.KeyTab, 0)
		if m.focus != want {
			t.Fatalf("tab focus = %d, want %d", m.focus, want)
		}
	}
	m, _ = pressKey(m, tea.KeyTab, tea.ModShift)
	if m.focus != focusClose {
		t.Fatalf("shift-tab focus = %d", m.focus)
	}
	m, _ = pressKey(m, tea.KeyTab, 0) // question
	m, _ = pressKey(m, tea.KeyDown, 0)
	if m.focus != focusSeats {
		t.Fatalf("down did not enter Seats: %d", m.focus)
	}
	m, _ = pressKey(m, tea.KeyDown, 0)
	if m.cursor != 1 {
		t.Fatalf("down did not move seat row: %d", m.cursor)
	}
	m, _ = pressKey(m, tea.KeyUp, 0)
	m, _ = pressKey(m, tea.KeyUp, 0)
	if m.focus != focusSeats || m.cursor != 0 {
		t.Fatalf("up at first row should keep Seats focused: %d", m.focus)
	}
	m, _ = pressKey(m, tea.KeyEnter, 0)
	if m.picked["p1"] {
		t.Fatal("Enter did not toggle focused seat")
	}
	m, _ = pressKey(m, tea.KeySpace, 0)
	if !m.picked["p1"] {
		t.Fatal("Space did not toggle focused seat")
	}
	m, _ = pressKey(m, tea.KeyTab, 0)
	m, _ = pressKey(m, tea.KeyEnter, 0)
	if !m.judgeOpen {
		t.Fatal("Enter did not open judge picker")
	}
	m, _ = pressKey(m, tea.KeyDown, 0)
	m, _ = pressKey(m, tea.KeyEnter, 0)
	if m.judgeOpen || m.judge != 2 {
		t.Fatalf("judge picker did not select: %d", m.judge)
	}
	m, _ = pressKey(m, tea.KeyLeft, 0)
	if m.judge != 1 {
		t.Fatalf("Left did not cycle judge back: %d", m.judge)
	}
	m, _ = pressKey(m, tea.KeyTab, 0) // Peer review
	m, _ = pressKey(m, tea.KeyTab, 0) // Ask button
	m, _ = pressKey(m, tea.KeyEnter, 0)
	if m.phase != watching || m.run == nil {
		t.Fatal("Enter on Ask did not submit")
	}
	q := askingModel(100, 32)
	q, _ = pressKey(q, tea.KeyEnter, 0)
	if q.phase != watching {
		t.Fatal("Enter in Question did not ask")
	}
	c := askingModel(100, 32)
	c.focus = focusClose
	_, cmd := pressKey(c, tea.KeyEnter, 0)
	if cmd == nil {
		t.Fatal("Enter on Close did not quit")
	}
}

func TestHelpAndContextHint(t *testing.T) {
	for _, size := range [][2]int{{100, 32}, {60, 24}} {
		footer := askingModel(size[0], size[1]).layout().lines[size[1]-1]
		if strings.Count(footer, "·") > 3 {
			t.Fatal("footer has more than four hints")
		}
		m := askingModel(size[0], size[1])
		questionBar := styleEscape.ReplaceAllString(strings.Join(m.layout().lines, "\n"), "")
		m, _ = pressKey(m, tea.KeyTab, 0)
		seatsBar := styleEscape.ReplaceAllString(strings.Join(m.layout().lines, "\n"), "")
		if questionBar == seatsBar || !strings.Contains(seatsBar, "? help") {
			t.Fatalf("%dx%d focus did not change: question=%q seats=%q", size[0], size[1], questionBar, seatsBar)
		}
		m, _ = pressKey(m, '?', 0)
		if !m.help || !strings.Contains(styleEscape.ReplaceAllString(strings.Join(m.layout().lines, "\n"), ""), "COUNCIL / HELP") {
			t.Fatal("? did not open help")
		}
		assertScreen(t, m)
		m, _ = pressKey(m, '?', 0)
		if m.help {
			t.Fatal("? did not close help")
		}
		m, _ = pressKey(m, '?', 0)
		m, _ = pressKey(m, tea.KeyEsc, 0)
		if m.help {
			t.Fatal("Esc did not close help")
		}
		m, _ = pressKey(m, '?', 0)
		m = clickAction(t, m, "help-close")
		if m.help {
			t.Fatal("help Close button did not close help")
		}
		w := watchingModel(size[0], size[1])
		watchBar := styleEscape.ReplaceAllString(strings.Join(w.layout().lines, "\n"), "")
		for _, key := range []string{"Verdict", "Copy", "Go to agent", "New question", "help"} {
			if !strings.Contains(watchBar, key) {
				t.Fatalf("watching legend missing %q", key)
			}
		}
		w, _ = pressKey(w, '?', 0)
		if !w.help {
			t.Fatal("? did not open watching help")
		}
		assertScreen(t, w)
	}
}

func clickAction(t *testing.T, m model, action string) model {
	t.Helper()
	for _, z := range m.layout().zones {
		if z.act == action {
			updated, _ := m.Update(tea.MouseClickMsg{X: z.x0 + (z.x1-z.x0)/2, Y: z.y, Button: tea.MouseLeft})
			return updated.(model)
		}
	}
	t.Fatalf("no zone for %s", action)
	return m
}

func TestUIClickActions(t *testing.T) {
	fakeAgents(t) // promptAgent is stubbed; no Herdr calls.
	for _, size := range [][2]int{{100, 32}, {60, 24}} {
		m := askingModel(size[0], size[1])
		m = clickAction(t, m, "toggle:0")
		if m.picked["p1"] {
			t.Fatal("toggle:0 did not deselect agy")
		}
		m = clickAction(t, m, "judge")
		if !m.judgeOpen {
			t.Fatal("judge click did not open picker")
		}
		m = clickAction(t, m, "judge-pick:2")
		if m.judge != 2 || m.judgeOpen {
			t.Fatalf("judge index = %d, want 2", m.judge)
		}
		m = clickAction(t, m, "ask")
		if m.phase != watching || m.run == nil || len(m.run.Seats) != 1 {
			t.Fatalf("ask failed: phase=%d run=%v", m.phase, m.run)
		}
		w := watchingModel(size[0], size[1])
		w = clickAction(t, w, "seat:0")
		if w.seat != 0 {
			t.Fatalf("seat:0 selected %d", w.seat)
		}
		w = clickAction(t, w, "seat:2")
		if !w.onVerdict() {
			t.Fatal("verdict tab was not selected")
		}
	}
}

func TestUISnapshots(t *testing.T) {
	ask := askingModel(100, 32)
	ask.input.SetValue("")
	for _, tc := range []struct {
		name string
		m    model
	}{
		{"ask", ask},
		{"watch", watchingModel(100, 32)},
	} {
		plain := styleEscape.ReplaceAllString(strings.Join(tc.m.layout().lines, "\n"), "")
		t.Logf("%s screen:\n%s", tc.name, plain)
	}
}

func stubWelcomeServices(t *testing.T, existing string) *int {
	t.Helper()
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", t.TempDir())
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	oldBind, oldCurrent := bindShortcutFn, currentShortcutFn
	calls := 0
	bindShortcutFn = func(want string) (string, bool, error) {
		calls++
		if want != "" {
			t.Fatalf("UI requested shortcut %q, want automatic choice", want)
		}
		return "prefix+a", false, nil
	}
	currentShortcutFn = func() string { return existing }
	t.Cleanup(func() { bindShortcutFn, currentShortcutFn = oldBind, oldCurrent })
	return &calls
}

func sizedLaunch(w, h int) model {
	m := launchModel("w1")
	m.w, m.h = w, h
	m.resize()
	return m
}

func TestWelcomeFirstRunAndPersistence(t *testing.T) {
	stubWelcomeServices(t, "")
	m := sizedLaunch(100, 32)
	if m.phase != welcoming || !m.settings.AutoTab || m.settings.Welcomed {
		t.Fatalf("first launch: phase=%d settings=%+v", m.phase, m.settings)
	}
	// Even a resumable run must wait behind the welcome screen.
	agents := uiAgents()
	r, err := newRun("prior question", agents[:1], agents[0])
	if err != nil {
		t.Fatal(err)
	}
	r.Seats[0].Sent = time.Now()
	r.save()
	if again := sizedLaunch(100, 32); again.phase != welcoming {
		t.Fatalf("recent run bypassed welcome: %d", again.phase)
	}
	m = clickAction(t, m, "welcome-auto")
	if m.settings.AutoTab || loadSettings().AutoTab {
		t.Fatal("welcome auto-tab toggle was not saved")
	}
	m = clickAction(t, m, "welcome-start")
	if m.phase != asking || !loadSettings().Welcomed || loadSettings().AutoTab {
		t.Fatalf("Start did not persist choices: phase=%d settings=%+v", m.phase, loadSettings())
	}
	if next := sizedLaunch(100, 32); next.phase == welcoming {
		t.Fatal("welcome repeated after Start")
	}
}

func TestWelcomeShortcutAndClicks(t *testing.T) {
	calls := stubWelcomeServices(t, "")
	for _, size := range [][2]int{{100, 32}, {60, 24}} {
		m := sizedLaunch(size[0], size[1])
		assertScreen(t, m)
		m = clickAction(t, m, "welcome-shortcut")
		if m.shortcut != "prefix+a" || !strings.Contains(m.preferenceNote, "prefix+a") {
			t.Fatalf("shortcut result not shown: %q %q", m.shortcut, m.preferenceNote)
		}
		for _, z := range m.layout().zones {
			if z.act == "welcome-shortcut" {
				t.Fatal("bound shortcut button still clickable")
			}
		}
		assertScreen(t, m)
	}
	if *calls != 2 {
		t.Fatalf("bind called %d times", *calls)
	}
	// An existing binding is visible immediately and never invokes the binder.
	currentShortcutFn = func() string { return "prefix+z" }
	m := sizedLaunch(100, 32)
	if !strings.Contains(styleEscape.ReplaceAllString(strings.Join(m.layout().lines, "\n"), ""), "Council is on prefix+z") {
		t.Fatal("existing shortcut missing")
	}
	m, _ = pressKey(m, tea.KeyTab, 0)
	if m.welcomeFocus != 2 {
		t.Fatal("focus did not skip disabled shortcut")
	}
}

func TestWelcomeKeyboardAndSettingsOverlay(t *testing.T) {
	stubWelcomeServices(t, "")
	for _, size := range [][2]int{{100, 32}, {60, 24}} {
		if err := saveSettings(Settings{AutoTab: true}); err != nil {
			t.Fatal(err)
		}
		m := sizedLaunch(size[0], size[1])
		m, _ = pressKey(m, tea.KeyTab, 0)
		if m.welcomeFocus != 1 {
			t.Fatal("Tab did not reach Add shortcut")
		}
		m, _ = pressKey(m, tea.KeyTab, tea.ModShift)
		if m.welcomeFocus != 0 {
			t.Fatal("Shift+Tab did not return to auto-tab")
		}
		m, _ = pressKey(m, tea.KeySpace, 0)
		if m.settings.AutoTab {
			t.Fatal("Space did not toggle auto-tab")
		}
		m, _ = pressKey(m, tea.KeyTab, 0)
		m, _ = pressKey(m, tea.KeyEnter, 0)
		if m.shortcut == "" {
			t.Fatal("Enter did not add shortcut")
		}
		m, _ = pressKey(m, tea.KeyEnter, 0) // shortcut success focuses Start
		if m.phase != asking {
			t.Fatal("Enter did not Start")
		}
		m = clickAction(t, m, "settings")
		if !m.settingsOpen {
			t.Fatal("Settings click did not open overlay")
		}
		assertScreen(t, m)
		m = clickAction(t, m, "settings-auto")
		if !m.settings.AutoTab || !loadSettings().AutoTab {
			t.Fatal("settings toggle was not saved")
		}
		m = clickAction(t, m, "settings-close")
		if m.settingsOpen {
			t.Fatal("Settings Close click failed")
		}
		m.focus = focusSeats
		m, _ = pressKey(m, 's', 0)
		if !m.settingsOpen {
			t.Fatal("s did not open Settings outside question")
		}
		m, _ = pressKey(m, tea.KeyEsc, 0)
		if m.settingsOpen {
			t.Fatal("Esc did not close Settings")
		}
	}
}

func TestWelcomeAndSettingsSnapshots(t *testing.T) {
	stubWelcomeServices(t, "")
	welcome := sizedLaunch(100, 32)
	settings := welcome.preferenceAction("welcome-start")
	settings.settingsOpen = true
	settings.settingsFocus = 0
	for _, tc := range []struct {
		name string
		m    model
	}{{"welcome", welcome}, {"settings", settings}} {
		plain := styleEscape.ReplaceAllString(strings.Join(tc.m.layout().lines, "\n"), "")
		t.Logf("%s screen:\n%s", tc.name, plain)
	}
	if _, err := os.Stat(filepath.Join(configDir(), "settings.json")); err != nil {
		t.Fatal(err)
	}
}

func manyAgents(n int) []Agent {
	result := make([]Agent, n)
	for i := range result {
		name := "agent"
		if i%3 == 0 {
			name = "claude"
		}
		result[i] = Agent{Name: name, Pane: fmt.Sprintf("pane-%02d", i), Title: fmt.Sprintf("terminal %02d", i), Status: "idle"}
	}
	return result
}

func manyAskModel(n, w, h int) model {
	m := newModel("w1")
	m.w, m.h = w, h
	m.resize()
	m.input.SetValue("How should we proceed?")
	m, _ = func() (model, tea.Cmd) {
		next, cmd := m.Update(agentsMsg{agents: manyAgents(n)})
		return next.(model), cmd
	}()
	return m
}

func manyWatchModel(n, w, h int) model {
	m := manyAskModel(n, w, h)
	m.phase = watching
	m.page = "run"
	m.run = &Run{Question: "How should we proceed?", Judge: &Seat{Agent: Agent{Name: "judge", Pane: "judge"}, State: Sending}, Seats: make([]*Seat, n)}
	for i, a := range m.agents {
		state := Working
		if i%3 == 0 {
			state = Done
		}
		m.run.Seats[i] = &Seat{Agent: a, State: state, Sent: time.Now().Add(-time.Minute), Answer: "An independent answer."}
	}
	m.seat = n - 1
	m.refreshAnswer()
	return m
}

func TestManyAgentLayoutsAndDefaults(t *testing.T) {
	for _, size := range [][2]int{{100, 32}, {60, 24}} {
		for _, n := range []int{1, 3, 12, 50} {
			m := manyAskModel(n, size[0], size[1])
			assertScreen(t, m)
			want := n <= 6
			for _, a := range m.agents {
				if m.picked[a.Pane] != want {
					t.Fatalf("%d idle default picked=%v want %v", n, m.picked[a.Pane], want)
				}
			}
			if n > 6 {
				plain := styleEscape.ReplaceAllString(strings.Join(m.layout().lines, "\n"), "")
				if !strings.Contains(plain, fmt.Sprintf("0 of %d", n)) {
					t.Fatal("large-list seat count missing")
				}
			}
			m.focus = focusSeats
			m.cursor = n - 1
			m.keepSeatVisible()
			assertScreen(t, m)
			top, end := m.visibleSeats()
			if n-1 < top || n-1 >= end {
				t.Fatalf("%d cursor invisible in [%d,%d)", n, top, end)
			}
			m = clickAction(t, m, fmt.Sprintf("toggle:%d", n-1))
			if m.picked[m.agents[n-1].Pane] == want {
				t.Fatalf("%d last row click did not toggle", n)
			}
			m = clickAction(t, m, "seat-all")
			if m.selectedCount() != n {
				t.Fatalf("all selected %d of %d", m.selectedCount(), n)
			}
			m = clickAction(t, m, "seat-none")
			if m.selectedCount() != 0 {
				t.Fatal("none did not clear")
			}
			m.focus = focusSeats
			m, _ = pressKey(m, 'a', 0)
			if m.selectedCount() != n {
				t.Fatal("a did not select all idle")
			}
			m, _ = pressKey(m, 'n', 0)
			if m.selectedCount() != 0 {
				t.Fatal("n did not clear seats")
			}
		}
	}
}

func TestManyConfirmationAndJudgePicker(t *testing.T) {
	fakeAgents(t)
	m := manyAskModel(12, 60, 24)
	m = clickAction(t, m, "seat-all")
	m = clickAction(t, m, "ask")
	if !m.confirmAsk || m.phase != asking {
		t.Fatal("more than 8 seats sent without confirmation")
	}
	assertScreen(t, m)
	m, _ = pressKey(m, tea.KeyEsc, 0)
	if m.confirmAsk || m.phase != asking {
		t.Fatal("Esc did not cancel confirmation")
	}
	m = clickAction(t, m, "judge")
	if !m.judgeOpen {
		t.Fatal("judge picker did not open")
	}
	assertScreen(t, m)
	for i := 0; i < 11; i++ {
		m, _ = pressKey(m, tea.KeyDown, 0)
	}
	m = clickAction(t, m, "judge-pick:11")
	if m.judge != 11 || m.judgeOpen {
		t.Fatal("picker did not select last judge")
	}
}

func TestManySeatWheel(t *testing.T) {
	m := manyAskModel(50, 60, 24)
	controlsY := -1
	for _, z := range m.layout().zones {
		if z.act == "seat-all" {
			controlsY = z.y
			break
		}
	}
	if controlsY < 0 {
		t.Fatal("seat controls missing")
	}
	if !m.wheelOnSeats(50, controlsY+1) {
		t.Fatal("seat region does not include blank space")
	}
	next, _ := m.Update(tea.MouseWheelMsg{X: 50, Y: controlsY + 1, Button: tea.MouseWheelDown})
	m = next.(model)
	if m.seatTop == 0 {
		t.Fatal("wheel did not scroll hovered seats")
	}
	m.focus = focusSeats
	m.cursor = m.seatTop
	next, _ = m.Update(tea.MouseWheelMsg{X: 50, Y: controlsY + 1, Button: tea.MouseWheelDown})
	m = next.(model)
	top, end := m.visibleSeats()
	if m.cursor < top || m.cursor >= end {
		t.Fatal("focused seat left viewport on wheel")
	}
}

func TestManyWatchingTabs(t *testing.T) {
	for _, size := range [][2]int{{100, 32}, {60, 24}} {
		for _, n := range []int{1, 3, 12, 50} {
			m := manyWatchModel(n, size[0], size[1])
			assertScreen(t, m)
			found := false
			for _, z := range m.layout().zones {
				if z.act == fmt.Sprintf("seat:%d", n-1) {
					found = true
				}
			}
			if !found {
				t.Fatalf("%d selected tab not visible at %dx%d", n, size[0], size[1])
			}
			m = clickAction(t, m, fmt.Sprintf("seat:%d", n))
			if !m.onVerdict() {
				t.Fatal("fixed verdict tab did not click")
			}
			m, _ = pressKey(m, '1', 0)
			m = clickAction(t, m, "seat:0")
			if m.seat != 0 {
				t.Fatal("first seat click failed")
			}
			if n > m.runListHeight() {
				m = clickAction(t, m, "tab-next")
				if m.seat == 0 {
					t.Fatal("next-page click did not advance")
				}
				before := m.seat
				m, _ = pressKey(m, ']', 0)
				if m.seat <= before {
					t.Fatal("] did not move to next page")
				}
				m, _ = pressKey(m, '[', 0)
				if m.seat >= before+1 {
					t.Fatal("[ did not move to previous page")
				}
			}
		}
	}
}

func TestManySnapshots(t *testing.T) {
	ask := manyAskModel(12, 100, 32)
	for i := range ask.agents {
		if i != 0 && i != 1 {
			ask.agents[i].Name = "agent"
		} else {
			ask.agents[i].Name = "claude"
		}
	}
	confirm := manyAskModel(12, 100, 32)
	for i := range confirm.agents {
		if i != 0 && i != 1 {
			confirm.agents[i].Name = "agent"
		} else {
			confirm.agents[i].Name = "claude"
		}
	}
	confirm = clickAction(t, confirm, "seat-all")
	confirm = clickAction(t, confirm, "ask")
	watch := manyWatchModel(50, 100, 32)
	for _, tc := range []struct {
		name string
		m    model
	}{{"ask-12", ask}, {"confirm-12", confirm}, {"watch-50", watch}} {
		assertScreen(t, tc.m)
		t.Logf("%s screen:\n%s", tc.name, styleEscape.ReplaceAllString(strings.Join(tc.m.layout().lines, "\n"), ""))
	}
}

func peerWatchModel(w, h int) model {
	m := manyWatchModel(3, w, h)
	m.run.PeerReview = true
	m.run.Order = []int{0, 1, 2}
	m.run.Letters = []string{"A", "B", "C"}
	m.run.Reviews = make([]*Seat, 3)
	for i := range m.run.Seats {
		m.run.Seats[i].State = Done
		m.run.Reviews[i] = &Seat{Agent: m.run.Seats[i].Agent, State: Working, Sent: time.Now().Add(-12 * time.Second)}
	}
	m.run.Reviews[0].State = Done
	m.run.Reviews[0].Answer = "Answer B is clearer.\nFINAL RANKING:\n1. Answer B\n2. Answer C"
	m.run.Ranking = []RankRow{{Letter: "B", Seat: 1, Avg: 1.0, Votes: 2}, {Letter: "A", Seat: 0, Avg: 2.0, Votes: 2}}
	m.seat = 0
	m.refreshAnswer()
	return m
}

func TestPeerDefaultsAndAsk(t *testing.T) {
	fakeAgents(t)
	for _, n := range []int{3, 6, 7, 50} {
		m := manyAskModel(n, 100, 32)
		if m.peerReview != (m.selectedCount() <= 6) {
			t.Fatalf("%d seats: peer default %v", n, m.peerReview)
		}
		m = clickAction(t, m, "seat-all")
		if m.peerReview != (n <= 6) {
			t.Fatalf("all %d: peer default %v", n, m.peerReview)
		}
		m = clickAction(t, m, "peer-toggle")
		chosen := m.peerReview
		m = clickAction(t, m, "seat-none")
		if !m.peerChosen || m.peerReview != chosen {
			t.Fatal("explicit peer choice changed with seats")
		}
		next, _ := m.do("new")
		m = next.(model)
		if !m.peerChosen || m.peerReview != chosen {
			t.Fatal("explicit peer choice lost on new question")
		}
	}
	m := askingModel(100, 32)
	m = clickAction(t, m, "peer-toggle")
	if m.peerReview {
		t.Fatal("mouse did not turn peer review off")
	}
	m = clickAction(t, m, "peer-toggle")
	m, _ = pressKey(m, tea.KeySpace, 0)
	if m.peerReview {
		t.Fatal("space did not toggle peer review")
	}
	m, _ = pressKey(m, tea.KeyEnter, 0)
	if !m.peerReview {
		t.Fatal("enter did not toggle peer review")
	}
	m = clickAction(t, m, "ask")
	if m.run == nil || !m.run.PeerReview {
		t.Fatal("ask did not pass peer review to run")
	}
}

func TestPeerStageAndRanking(t *testing.T) {
	m := peerWatchModel(100, 32)
	if line := m.stageLine(); !strings.Contains(line, "Peer review 1/3") || !strings.Contains(line, "Verdict") {
		t.Fatalf("review stage: %q", line)
	}
	m.run.PeerReview = false
	if line := m.stageLine(); strings.Contains(line, "Peer review") || !strings.Contains(line, "Verdict") {
		t.Fatalf("no-review stage: %q", line)
	}
	m.run.PeerReview = true
	m.run.Reviews = nil
	m.run.Seats[2].State = Working
	if line := m.stageLine(); strings.Contains(line, "Peer review") || !strings.Contains(line, "Verdict") {
		t.Fatalf("fewer than three answers: %q", line)
	}
	m.run.Seats[2].State = Done
	m.run.Reviews = make([]*Seat, 3)
	m.run.Judge.State = Working
	m.seat = len(m.run.Seats)
	m.refreshAnswer()
	plain := styleEscape.ReplaceAllString(strings.Join(m.layout().lines, "\n"), "")
	if !strings.Contains(plain, "PEER RANKING") || strings.Contains(m.currentText(), "(agent") || !strings.Contains(m.currentText(), "Answer B") {
		t.Fatalf("pending ranking leaked names or missing: %q", m.currentText())
	}
	m.run.Judge.State = Done
	m.run.Judge.Answer = "Use the strongest answer."
	m.refreshAnswer()
	if !strings.Contains(m.currentText(), "(agent") || !strings.Contains(m.currentText(), "Use the strongest answer") {
		t.Fatalf("revealed copy missing ranking/verdict: %q", m.currentText())
	}
}

func TestPeerReviewSwitchAndLayouts(t *testing.T) {
	for _, size := range [][2]int{{60, 24}, {100, 32}} {
		for _, n := range []int{3, 50} {
			ask := manyAskModel(n, size[0], size[1])
			assertScreen(t, ask)
			ask = clickAction(t, ask, "peer-toggle")
			assertScreen(t, ask)
			watch := manyWatchModel(n, size[0], size[1])
			watch.run.PeerReview = true
			watch.run.Reviews = make([]*Seat, n)
			watch.run.Reviews[0] = &Seat{Agent: watch.run.Seats[0].Agent, State: Done, Answer: "A review"}
			watch.seat = 0
			watch.refreshAnswer()
			assertScreen(t, watch)
			watch = clickAction(t, watch, "review")
			if !watch.showReview || watch.currentText() != "A review" {
				t.Fatal("review click did not show review")
			}
			watch = clickAction(t, watch, "answer")
			if watch.showReview {
				t.Fatal("answer click did not restore answer")
			}
			watch, _ = pressKey(watch, 'r', 0)
			if !watch.showReview {
				t.Fatal("r did not switch to review")
			}
			watch.seat = n
			watch.run.Judge.State = Done
			watch.run.Judge.Answer = "Verdict"
			watch.run.Ranking = []RankRow{{Letter: "A", Seat: 0, Avg: 1, Votes: 1}}
			watch.refreshAnswer()
			assertScreen(t, watch)
		}
	}
}

func TestPeerSnapshots(t *testing.T) {
	ask := manyAskModel(3, 100, 32)
	watch := peerWatchModel(100, 32)
	verdict := peerWatchModel(100, 32)
	verdict.seat = len(verdict.run.Seats)
	verdict.run.Judge.State = Done
	verdict.run.Judge.Sent = time.Now().Add(-10 * time.Second)
	verdict.run.Judge.Answer = "Use the modular approach."
	for _, rv := range verdict.run.Reviews {
		rv.State = Done
	}
	verdict.refreshAnswer()
	for _, tc := range []struct {
		name string
		m    model
	}{{"peer-ask", ask}, {"peer-watch", watch}, {"peer-verdict", verdict}} {
		t.Logf("%s screen:\n%s", tc.name, styleEscape.ReplaceAllString(strings.Join(tc.m.layout().lines, "\n"), ""))
	}
}

// clickText uses the rendered cells, independently of the action-zone definitions.
func clickText(t *testing.T, m model, label string) (model, tea.Cmd) {
	t.Helper()
	for y, line := range m.layout().lines {
		plain := styleEscape.ReplaceAllString(line, "")
		if at := strings.Index(plain, label); at >= 0 {
			x := lipgloss.Width(plain[:at]) + lipgloss.Width(label)/2
			next, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
			return next.(model), cmd
		}
	}
	t.Fatalf("rendered label %q missing", label)
	return m, nil
}

func TestNavigationClicksAndKeyboard(t *testing.T) {
	stubWelcomeServices(t, "prefix+a")
	for _, size := range [][2]int{{60, 24}, {100, 32}, {140, 40}} {
		m := watchingModel(size[0], size[1])
		run := m.run
		m, _ = clickText(t, m, "Ask")
		if m.page != "ask" || m.run != run || !m.input.Focused() {
			t.Fatal("Ask navigation lost run or input focus")
		}
		before := m.input.Value()
		typed, _ := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
		m = typed.(model)
		if m.input.Value() == before {
			t.Fatal("typing on Ask was routed to run shortcuts")
		}
		m, _ = pressKey(m, tea.KeyRight, tea.ModCtrl)
		if m.page != "runs" {
			t.Fatal("Ctrl+right did not reach Runs")
		}
		m, _ = pressKey(m, tea.KeyRight, tea.ModCtrl)
		if m.page != "settings" || !m.settingsOpen {
			t.Fatal("Ctrl+right did not reach Settings")
		}
		m, _ = pressKey(m, tea.KeyRight, tea.ModCtrl)
		if m.page != "run" || m.settingsOpen {
			t.Fatal("Ctrl+right did not reach current run")
		}
		m, _ = pressKey(m, tea.KeyRight, tea.ModCtrl)
		if m.page != "ask" {
			t.Fatal("page cycle did not wrap")
		}
		m, _ = clickText(t, m, "Runs")
		if m.page != "runs" {
			t.Fatal("Runs nav click failed")
		}
		m, _ = clickText(t, m, "Settings")
		if m.page != "settings" {
			t.Fatal("Settings nav click failed")
		}
		m, _ = pressKey(m, tea.KeyLeft, tea.ModCtrl)
		if m.page != "runs" {
			t.Fatal("Ctrl+left did not work from Settings")
		}
		m = clickAction(t, m, "nav-run")
		if m.page != "run" {
			t.Fatal("Current run nav click failed")
		}
		assertScreen(t, m)
	}
}

func storedUIRun(t *testing.T, question, workspace string, sent time.Time) *Run {
	t.Helper()
	r, err := newRun(question, uiAgents(), uiAgents()[1])
	if err != nil {
		t.Fatal(err)
	}
	r.Workspace, r.PeerReview = workspace, true
	for i, s := range r.Seats {
		s.Sent = sent
		s.State = Done
		answer(s, fmt.Sprintf("Independent answer %d.", i+1))
	}
	r.save()
	return r
}

func TestRunsNewestFirstOpenAndNoRedispatch(t *testing.T) {
	sent := fakeAgents(t)
	older := storedUIRun(t, "Older question", "w1", time.Now().Add(-2*time.Hour))
	newer := storedUIRun(t, "Newest question", "w1", time.Now().Add(-time.Hour))
	storedUIRun(t, "Other workspace", "w2", time.Now())
	m := askingModel(100, 32)
	m = clickAction(t, m, "nav-runs")
	if len(m.runs) != 2 || m.runs[0].ID != newer.ID || m.runs[1].ID != older.ID {
		t.Fatal("Runs did not load workspace history newest-first")
	}
	plain := styleEscape.ReplaceAllString(strings.Join(m.layout().lines, "\n"), "")
	if strings.Index(plain, "Newest question") > strings.Index(plain, "Older question") || strings.Contains(plain, "Other workspace") {
		t.Fatal("history render order or workspace filter wrong")
	}
	m = clickAction(t, m, "open-run:0")
	if m.page != "run" || m.run.ID != newer.ID || !m.oldRun || m.currentText() != "Independent answer 1." {
		t.Fatal("click did not open saved answer")
	}
	for i := 0; i < 3; i++ {
		next, cmd := m.Update(agentsMsg{agents: uiAgents()})
		m = next.(model)
		if cmd != nil {
			t.Fatal("viewing old run scheduled a prompt")
		}
	}
	m, _ = pressKey(m, 'j', 0)
	if !m.run.Judge.Sent.IsZero() || len(*sent) != 0 {
		t.Fatal("history viewing prompted agents or judge")
	}
	m = clickAction(t, m, "nav-runs")
	m, _ = pressKey(m, tea.KeyDown, 0)
	m, _ = pressKey(m, tea.KeyEnter, 0)
	if m.run.ID != older.ID {
		t.Fatal("Enter did not open selected historical run")
	}
	assertScreen(t, m)
}

func TestEmptyRunsAndHistoryWheel(t *testing.T) {
	fakeAgents(t)
	m := askingModel(60, 24)
	m = clickAction(t, m, "nav-runs")
	if !strings.Contains(strings.Join(m.layout().lines, "\n"), "No questions yet in this workspace.") {
		t.Fatal("empty history message missing")
	}
	m, _ = pressKey(m, tea.KeyDown, 0)
	if m.runCursor != 0 {
		t.Fatal("empty history cursor became invalid")
	}
	for i := 0; i < 30; i++ {
		m.runs = append(m.runs, &Run{Question: fmt.Sprintf("Question %02d", i), Judge: &Seat{}, Seats: []*Seat{{}}})
	}
	next, _ := m.Update(tea.MouseWheelMsg{X: 30, Y: 5, Button: tea.MouseWheelDown})
	m = next.(model)
	if m.runTop == 0 || m.runCursor < m.runTop {
		t.Fatal("history wheel did not scroll and retain a visible selection")
	}
	plain := styleEscape.ReplaceAllString(strings.Join(m.layout().lines, "\n"), "")
	if strings.Contains(plain, "Question 00") {
		t.Fatal("wheel scrolled state without scrolling rendered history")
	}
	assertScreen(t, m)
}

func TestPrimaryDisabledAndComposedClick(t *testing.T) {
	fakeAgents(t)
	for _, size := range [][2]int{{60, 24}, {100, 32}, {140, 40}} {
		for _, missing := range []string{"question", "seats"} {
			m := askingModel(size[0], size[1])
			if missing == "question" {
				m.input.SetValue("")
			} else {
				m.picked = map[string]bool{}
			}
			m, cmd := clickText(t, m, "Ask council")
			if m.run != nil || m.phase != asking || cmd != nil {
				t.Fatalf("disabled primary launched with missing %s", missing)
			}
		}
		m := askingModel(size[0], size[1])
		m, _ = clickText(t, m, "● agy")
		if m.picked["p1"] {
			t.Fatal("drawn agent pill click missed composed zone")
		}
		m = clickAction(t, m, "judge")
		if !m.judgeOpen {
			t.Fatal("judge dropdown did not open")
		}
		if !strings.Contains(strings.Join(m.layout().lines, "\n"), "Pick judge") {
			t.Fatal("rounded picker missing")
		}
		m = clickAction(t, m, "judge-pick:1")
		m, _ = clickText(t, m, "[ ON ]")
		if m.peerReview {
			t.Fatal("drawn peer switch click missed composed zone")
		}
		m, cmd := clickText(t, m, "Ask council")
		if m.phase != watching || m.run == nil || cmd == nil || len(m.run.Seats) != 1 {
			t.Fatal("click at drawn Ask council coordinates did not start run")
		}
	}
}

func TestRedesignSizeMatrix(t *testing.T) {
	stubWelcomeServices(t, "")
	for _, size := range [][2]int{{60, 24}, {100, 32}, {140, 40}} {
		for _, n := range []int{3, 50} {
			ask := manyAskModel(n, size[0], size[1])
			ask.focus, ask.cursor = focusSeats, n/2
			ask.keepSeatVisible()
			watch := manyWatchModel(n, size[0], size[1])
			settings := ask
			settings.page, settings.settingsOpen = "settings", true
			runs := ask
			runs.page = "runs"
			welcome := ask
			welcome.phase = welcoming
			for _, m := range []model{ask, watch, settings, runs, welcome} {
				assertScreen(t, m)
				if len(m.layout().lines) != m.h {
					t.Fatal("frame does not fill pane height")
				}
				for _, line := range m.layout().lines {
					if lipgloss.Width(line) != m.w {
						t.Fatalf("frame row does not fill width: %q", line)
					}
				}
			}
			for _, action := range []string{"ask", "judge", "peer-toggle", fmt.Sprintf("toggle:%d", n/2)} {
				found := false
				for _, z := range ask.layout().zones {
					if z.act == action {
						found = true
					}
				}
				if !found {
					t.Fatalf("%dx%d / %d agents: %s was clipped", size[0], size[1], n, action)
				}
			}
			for _, action := range []string{"copy", "goto", "new", fmt.Sprintf("seat:%d", n-1), fmt.Sprintf("seat:%d", n)} {
				found := false
				for _, z := range watch.layout().zones {
					if z.act == action {
						found = true
					}
				}
				if !found {
					t.Fatalf("%dx%d / %d agents: %s was clipped", size[0], size[1], n, action)
				}
			}
		}
	}
}

func TestAnswerWrappingAndViewportScroll(t *testing.T) {
	for _, size := range [][2]int{{60, 24}, {100, 32}, {140, 40}} {
		m := watchingModel(size[0], size[1])
		m.seat = 0
		body := strings.Repeat("The loop stops after every item has been checked; preserve every word.\n", 50) + "FINAL-TOKEN"
		m.run.Seats[0].Answer = body
		m.refreshAnswer()
		vp := m.answer
		vp.SetHeight(1000)
		plain := styleEscape.ReplaceAllString(vp.View(), "")
		if strings.Join(strings.Fields(plain), " ") != strings.Join(strings.Fields(body), " ") {
			t.Fatal("right pane wrapping lost answer text")
		}
		for i := 0; i < 100; i++ {
			next, _ := m.Update(tea.MouseWheelMsg{X: m.w - 10, Y: 10, Button: tea.MouseWheelDown})
			m = next.(model)
		}
		if !strings.Contains(strings.Join(m.layout().lines, "\n"), "FINAL-TOKEN") {
			t.Fatal("wheel could not reach the final answer line")
		}
		assertScreen(t, m)
	}
}

func TestRedesignSnapshots(t *testing.T) {
	stubWelcomeServices(t, "prefix+a")
	ask := askingModel(100, 32)
	ask.picked = map[string]bool{"p1": true, "p2": true, "p3": true}
	ask.input.SetValue("Is retry.ts correct, and what should we change?")
	now := time.Now()
	watch := ask
	watch.phase, watch.page = watching, "run"
	watch.run = &Run{Question: ask.input.Value(), Judge: &Seat{Agent: ask.agents[1], State: Sending}, Order: []int{0, 1, 2}, PeerReview: true}
	for i, a := range ask.agents {
		watch.run.Seats = append(watch.run.Seats, &Seat{Agent: a, State: Working, Sent: now.Add(-time.Duration(42-i*12) * time.Second)})
	}
	watch.run.Seats[0].State, watch.run.Seats[0].Finished = Done, now
	watch.run.Seats[0].Answer = "The retry loop executes one extra time because it uses i <= attempts.\n\nUse i < attempts and keep the final error as the cause."
	watch.run.Seats[2].State = Blocked
	watch.refreshAnswer()
	verdict := ask
	verdict.phase, verdict.page = watching, "run"
	verdict.run = &Run{Question: ask.input.Value(), Judge: &Seat{Agent: ask.agents[1], State: Done, Sent: now.Add(-12 * time.Second), Finished: now, Answer: "## Consensus\nUse a strict loop bound and preserve the final error.\n\n## Final answer\nReplace i <= attempts with i < attempts. Add a test for exactly three calls when attempts is 3."}, Order: []int{0, 1, 2}, Letters: []string{"A", "B", "C"}, PeerReview: true}
	for i, a := range ask.agents {
		verdict.run.Seats = append(verdict.run.Seats, &Seat{Agent: a, State: Done, Sent: now.Add(-42 * time.Second), Finished: now.Add(-time.Duration(i*5) * time.Second)})
		verdict.run.Reviews = append(verdict.run.Reviews, &Seat{Agent: a, State: Done, Sent: now.Add(-20 * time.Second), Finished: now.Add(-10 * time.Second)})
	}
	verdict.seat = len(verdict.run.Seats)
	verdict.refreshAnswer()
	runs := ask
	runs.page = "runs"
	historyDone := *verdict.run
	historyWaiting := *watch.run
	historyDone.Seats = append([]*Seat(nil), verdict.run.Seats...)
	historyWaiting.Seats = append([]*Seat(nil), watch.run.Seats...)
	firstDone := *historyDone.Seats[0]
	firstWaiting := *historyWaiting.Seats[0]
	firstDone.Sent = time.Date(2026, 10, 7, 13, 5, 0, 0, time.Local)
	firstWaiting.Sent = time.Date(2026, 10, 7, 12, 42, 0, 0, time.Local)
	historyDone.Seats[0], historyWaiting.Seats[0] = &firstDone, &firstWaiting
	historyWaiting.Question = "Should we keep the retry policy inside the client?"
	runs.runs = []*Run{&historyDone, &historyWaiting}
	settings := ask
	settings.page, settings.settingsOpen, settings.settings.AutoTab = "settings", true, true
	settings.shortcut = "prefix+a"
	narrow := ask
	narrow.w, narrow.h = 60, 24
	narrow.resize()
	var snapshots strings.Builder
	for _, tc := range []struct {
		name string
		m    model
	}{
		{"Ask — 3 agents, 100×32", ask}, {"Run — answers in progress, 100×32", watch},
		{"Run — verdict, 100×32", verdict}, {"Runs — 100×32", runs},
		{"Settings — 100×32", settings}, {"Ask — 3 agents, 60×24", narrow},
	} {
		assertScreen(t, tc.m)
		plain := styleEscape.ReplaceAllString(strings.Join(tc.m.layout().lines, "\n"), "")
		t.Logf("%s\n%s", tc.name, plain)
		fmt.Fprintf(&snapshots, "## %s\n\n```text\n%s\n```\n\n", tc.name, plain)
	}
	if path := os.Getenv("COUNCIL_SNAPSHOT_FILE"); path != "" {
		if err := os.WriteFile(path, []byte(snapshots.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestJudgeNowAndSettingsReturnToRun(t *testing.T) {
	sent := fakeAgents(t)
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", t.TempDir())
	m := askingModel(60, 24)
	m.run = storedUIRun(t, "Compare these approaches", "w1", time.Now().Add(-time.Minute))
	for _, s := range m.run.Seats {
		s.State = Done
		s.Answer = "A complete answer."
	}
	m.phase, m.page = watching, "run"
	m.refreshAnswer()
	m = clickAction(t, m, "settings")
	m = clickAction(t, m, "settings-close")
	if m.page != "run" || m.settingsOpen {
		t.Fatal("Settings Back lost the current run page")
	}
	m, _ = clickText(t, m, "Judge now")
	if !m.onVerdict() || m.run.Judge.Sent.IsZero() || len(*sent) != 1 {
		t.Fatal("Judge now did not start the judge via the stub")
	}
	assertScreen(t, m)
}
