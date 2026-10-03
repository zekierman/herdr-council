package main

import (
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
	if m.judge != 2 {
		t.Fatalf("Enter did not cycle judge: %d", m.judge)
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
		if m.judge != 2 {
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
