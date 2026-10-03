package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func (m model) seatSummary() string {
	return fmt.Sprintf("%d of %d selected · each answers independently", m.selectedCount(), len(m.agents))
}

func (m model) seatHint() string {
	if len(m.agents) > 6 && m.selectedCount() == 0 {
		return "select seats: a = all idle"
	}
	return "a = all idle · n = none"
}

func (m model) seatListHeight() int {
	// The rest of Ask uses at most 19 rows at narrow sizes; keep a viewport in the middle.
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
	row := fmt.Sprintf("%-10s %-9s", shorten(a.Name, 10), shorten(a.Status, 9))
	if hint != "" {
		row += " · " + hint
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
	border := dim.Render("+" + strings.Repeat("-", panelWidth-2) + "+")
	panel.add(seg(border))
	panel.add(seg("| " + paddedCell(bold.Render("PICK JUDGE"), panelWidth-4) + " |"))
	panel.add(seg("| " + paddedCell(dim.Render("Choose one agent · ↑↓ move · enter select"), panelWidth-4) + " |"))
	panel.add(seg(border))
	top := min(max(0, m.judgeTop), max(0, len(m.agents)-m.judgePickerHeight()))
	if m.judgeCursor < top {
		top = m.judgeCursor
	}
	if m.judgeCursor >= top+m.judgePickerHeight() {
		top = m.judgeCursor - m.judgePickerHeight() + 1
	}
	end := min(len(m.agents), top+m.judgePickerHeight())
	if top > 0 {
		panel.add(seg("| " + paddedCell(dim.Render(fmt.Sprintf("↑ %d more", top)), panelWidth-4) + " |"))
	}
	for i := top; i < end; i++ {
		mark := askFocusMark(i == m.judgeCursor)
		label := mark + shorten(fmt.Sprintf("%d  %s", i+1, m.agentRow(i, panelWidth-12)), panelWidth-6)
		panel.add(seg("| "), act(paddedCell(label, panelWidth-4), fmt.Sprintf("judge-pick:%d", i)), seg(" |"))
	}
	if end < len(m.agents) {
		panel.add(seg("| " + paddedCell(dim.Render(fmt.Sprintf("↓ %d more", len(m.agents)-end)), panelWidth-4) + " |"))
	}
	panel.add(seg(border))
	panel.add(seg("| "), act(paddedCell(button.Render("Cancel")+"  esc", panelWidth-4), "judge-cancel"), seg(" |"))
	panel.add(seg(border))
	return placeOverlay(base, panel, width, height)
}

func (m model) statusSummary() string {
	var done, working, waiting, noAnswer int
	for _, s := range m.run.Seats {
		switch s.State {
		case Done:
			done++
		case Working, Sending:
			working++
		case Blocked:
			waiting++
		default:
			noAnswer++
		}
	}
	return fmt.Sprintf("%d done · %d working · %d waiting · %d no answer", done, working, waiting, noAnswer)
}

func compactTab(s *Seat, i, selected int) string {
	state := "·"
	switch s.State {
	case Done:
		state = "✓"
	case Working, Sending:
		state = "◌"
	case Blocked:
		state = "!"
	case Silent, Late:
		state = "?"
	case Failed:
		state = "×"
	}
	label := fmt.Sprintf("[%d %s %s]", i+1, shorten(s.Agent.Name, 7), state)
	if i == selected {
		return accent.Render(label)
	}
	return dim.Render(label)
}

func (m model) tabWindow() (int, int) {
	n := len(m.run.Seats)
	if n == 0 {
		return 0, 0
	}
	start := min(max(0, m.tabStart), n-1)
	if m.seat < n && m.seat < start {
		start = m.seat
	}
	for {
		end := m.tabEnd(start)
		if m.seat >= n || m.seat < end || start >= n-1 {
			return start, end
		}
		start++
	}
}

func (m model) tabEnd(start int) int {
	n := len(m.run.Seats)
	// Reserve space for the verdict and both overflow indicators.
	space := max(12, m.w-34)
	used := 0
	end := start
	for end < n {
		w := lipgloss.Width(compactTab(m.run.Seats[end], end, m.seat)) + 1
		if end > start && used+w > space {
			break
		}
		used += w
		end++
	}
	return end
}

func (m *model) keepTabVisible() {
	if m.run == nil || len(m.run.Seats) == 0 {
		return
	}
	start, _ := m.tabWindow()
	m.tabStart = start
}

func (m model) tabPageSize() int {
	if m.run == nil {
		return 1
	}
	start, end := m.tabWindow()
	return max(1, end-start)
}
