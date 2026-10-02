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
	for _, size := range [][2]int{{100, 32}, {60, 24}} {
		for _, m := range []model{askingModel(size[0], size[1]), watchingModel(size[0], size[1])} {
			assertScreen(t, m)
		}
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
