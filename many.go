package main

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m model) selectedCount() int {
	n := 0
	for _, a := range m.agents {
		if m.picked[a.Pane] {
			n++
		}
	}
	return n
}

func (m model) seatListHeight() int {
	return max(1, min(12, m.h-21))
}

func (m *model) keepSeatVisible() {
	h := m.seatListHeight()
	m.seatTop = min(max(0, m.seatTop), max(0, len(m.agents)-h))
	if m.cursor < m.seatTop {
		m.seatTop = m.cursor
	}
	if m.cursor >= m.seatTop+h {
		m.seatTop = m.cursor - h + 1
	}
}

func (m model) visibleSeats() (int, int) {
	h := m.seatListHeight()
	top := min(max(0, m.seatTop), max(0, len(m.agents)-h))
	if m.focus == focusSeats && len(m.agents) > 0 {
		if m.cursor < top {
			top = m.cursor
		}
		if m.cursor >= top+h {
			top = m.cursor - h + 1
		}
	}
	return top, min(len(m.agents), top+h)
}

func (m model) wheelOnSeats(x, y int) bool {
	if x < 0 || x >= m.w {
		return false
	}
	controlsY, judgeY := -1, -1
	for _, z := range m.layout().zones {
		if z.act == "seat-all" {
			controlsY = z.y
		}
		if z.act == "judge" {
			judgeY = z.y
		}
	}
	return controlsY >= 0 && judgeY > controlsY && y >= controlsY && y < judgeY
}

func (m model) agentRow(i, width int) string {
	a := m.agents[i]
	count := 0
	for _, other := range m.agents {
		if other.Name == a.Name {
			count++
		}
	}
	hint := ""
	if count > 1 {
		hint = a.Pane
		if hint == "" {
			hint = a.Title
		}
	}
	row := fit(shorten(agentName(a.Name), 10), 10) + " " + fit(dim.Render(shorten(a.Status, 9)), 9)
	if hint != "" {
		row += dim.Render(" · " + hint)
	}
	return shorten(row, width)
}

func (m model) judgePickerHeight() int { return max(1, min(12, m.h-8)) }

func (m *model) keepJudgeVisible() {
	h := m.judgePickerHeight()
	m.judgeTop = min(max(0, m.judgeTop), max(0, len(m.agents)-h))
	if m.judgeCursor < m.judgeTop {
		m.judgeTop = m.judgeCursor
	}
	if m.judgeCursor >= m.judgeTop+h {
		m.judgeTop = m.judgeCursor - h + 1
	}
}

func (m model) updateJudgePicker(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		return m.do("judge-cancel")
	case "up", "k":
		m.judgeCursor = max(0, m.judgeCursor-1)
	case "down", "j":
		m.judgeCursor = min(len(m.agents)-1, m.judgeCursor+1)
	case "pgup":
		m.judgeCursor = max(0, m.judgeCursor-m.judgePickerHeight())
	case "pgdown":
		m.judgeCursor = min(len(m.agents)-1, m.judgeCursor+m.judgePickerHeight())
	case "enter", "space":
		return m.do(fmt.Sprintf("judge-pick:%d", m.judgeCursor))
	}
	m.keepJudgeVisible()
	return m, nil
}

func (m model) judgePickerOverlay(base screen) screen {
	width, height := max(20, m.w), max(10, m.h)
	panelWidth := min(54, width-4)
	var panel screen
	panel.add(seg(dim.Render("Choose one agent · ↑↓ move · enter select")))
	top := min(max(0, m.judgeTop), max(0, len(m.agents)-m.judgePickerHeight()))
	if m.judgeCursor < top {
		top = m.judgeCursor
	}
	if m.judgeCursor >= top+m.judgePickerHeight() {
		top = m.judgeCursor - m.judgePickerHeight() + 1
	}
	end := min(len(m.agents), top+m.judgePickerHeight())
	if top > 0 {
		panel.add(seg(dim.Render(fmt.Sprintf("↑ %d more", top))))
	}
	for i := top; i < end; i++ {
		mark := askFocusMark(i == m.judgeCursor)
		label := mark + shorten(fmt.Sprintf("%d  %s", i+1, m.agentRow(i, panelWidth-12)), panelWidth-6)
		panel.add(act(paddedCell(label, panelWidth-4), fmt.Sprintf("judge-pick:%d", i)))
	}
	if end < len(m.agents) {
		panel.add(seg(dim.Render(fmt.Sprintf("↓ %d more", len(m.agents)-end))))
	}
	panel.blank()
	panel.add(act(button.Render("Cancel"), "judge-cancel"))
	return placeOverlay(base, roundedPanel(panel, panelWidth, "Pick judge"), width, height)
}
