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
	if !strings.Contains(wide, strings.TrimSpace(councilArt[3])) || strings.Contains(narrow, strings.TrimSpace(councilArt[3])) {
		t.Fatal("art should appear only where it fits")
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
	for _, want := range []askFocus{focusSeats, focusJudge, focusAsk, focusClose, focusQuestion} {
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
	if m.focus != focusQuestion {
		t.Fatalf("up at first row did not return to Question: %d", m.focus)
	}
	m, _ = pressKey(m, tea.KeyTab, 0)
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
		for focus := focusQuestion; focus < focusCount; focus++ {
			if rows := askLegend(focus, size[0]); len(rows) != 1 {
				t.Fatalf("%dx%d focus %d has %d hint rows", size[0], size[1], focus, len(rows))
			}
		}
		m := askingModel(size[0], size[1])
		questionLines := m.layout().lines
		questionBar := styleEscape.ReplaceAllString(strings.Join(questionLines[max(0, len(questionLines)-2):], "\n"), "")
		m, _ = pressKey(m, tea.KeyTab, 0)
		seatLines := m.layout().lines
		seatsBar := styleEscape.ReplaceAllString(strings.Join(seatLines[max(0, len(seatLines)-2):], "\n"), "")
		if questionBar == seatsBar || !strings.Contains(seatsBar, "select") || !strings.Contains(seatsBar, "help") {
			t.Fatalf("%dx%d context bar did not change: question=%q seats=%q", size[0], size[1], questionBar, seatsBar)
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
		for _, key := range []string{"←→/tab", "1-9", "verdict", "wheel", "copy", "agent", "judge", "new", "close", "help"} {
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
	m.run = &Run{Question: "How should we proceed?", Judge: &Seat{Agent: Agent{Name: "judge", Pane: "judge"}, State: Sending}, Seats: make([]*Seat, n)}
	for i, a := range m.agents {
		state := Working
		if i%3 == 0 {
			state = Done
		}
		m.run.Seats[i] = &Seat{Agent: a, State: state, Sent: time.Now().Add(-time.Minute), Answer: "An independent answer."}
	}
	m.seat = n - 1
	m.keepTabVisible()
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
				if !strings.Contains(plain, "select seats: a = all idle") {
					t.Fatal("large-list selection hint missing")
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
			if n > 3 {
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
