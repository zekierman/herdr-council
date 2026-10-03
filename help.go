package main

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var keycap = lipgloss.NewStyle().Foreground(lipgloss.Color("#e6edf3")).Background(lipgloss.Color("#30363d")).Padding(0, 1)

func askFocusMark(active bool) string {
	if active {
		return accent.Render("› ")
	}
	return "  "
}

func addAskHeading(sc *screen, focused bool, number, title, key, action string, width int) {
	left := askFocusMark(focused) + accent.Render(number) + "  " + bold.Render(title)
	right := keycap.Render(key) + " " + dim.Render(action)
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	sc.add(seg(left), seg(dim.Render(strings.Repeat("·", gap-1)+" ")), seg(right))
}

type legendItem struct {
	key, meaning, action string
}

func keyLegend(items []legendItem, width int) [][][2]string {
	var rows [][][2]string
	row := [][2]string{seg(" ")}
	used := 1
	for _, item := range items {
		label := keycap.Render(item.key) + " " + dim.Render(item.meaning)
		gap := 0
		if used > 1 {
			gap = 2
		}
		if used > 1 && used+gap+lipgloss.Width(label) > width {
			rows = append(rows, row)
			row = [][2]string{seg(" ")}
			used, gap = 1, 0
		}
		if gap > 0 {
			row = append(row, seg("  "))
			used += 2
		}
		if item.action != "" {
			row = append(row, act(label, item.action))
		} else {
			row = append(row, seg(label))
		}
		used += lipgloss.Width(label)
	}
	return append(rows, row)
}

func askLegend(focus askFocus, width int) [][][2]string {
	items := []legendItem{{"tab", "next", ""}}
	switch focus {
	case focusQuestion:
		items = append(items, legendItem{"S-tab", "back", ""}, legendItem{"enter", "ask", ""})
	case focusSeats:
		items = append(items, legendItem{"↑↓", "row", ""}, legendItem{"space/enter", "select", ""})
	case focusJudge:
		items = append(items, legendItem{"S-tab", "back", ""}, legendItem{"←→/enter", "cycle", ""})
	case focusAsk:
		items = append(items, legendItem{"S-tab", "back", ""}, legendItem{"enter", "send", ""})
	case focusClose:
		items = append(items, legendItem{"S-tab", "back", ""}, legendItem{"enter", "close", ""})
	}
	return keyLegend(append(items, legendItem{"?", "help", "help"}), width)
}

func watchLegend(width int) [][][2]string {
	return keyLegend([]legendItem{
		{"←→/tab", "switch", ""}, {"1-9", "seat", ""}, {"v", "verdict", ""},
		{"↑↓/wheel", "scroll", ""}, {"c", "copy", ""}, {"f", "agent", ""},
		{"j", "judge", ""}, {"n", "new", ""}, {"q", "close", ""}, {"?", "help", "help"},
	}, width)
}

type helpEntry struct{ key, meaning string }

var askHelp = []helpEntry{
	{"tab/S-tab", "focus next/prev"},
	{"↑↓", "section / row"},
	{"k/j", "seat row"},
	{"enter", "use focused"},
	{"space/x", "select seat"},
	{"←→", "cycle judge"},
	{"J", "cycle judge"},
	{"?", "toggle help"},
	{"esc", "close"},
	{"ctrl+c", "quit"},
	{"mouse", "fields/buttons"},
}

var watchHelp = []helpEntry{
	{"←→/tab", "switch tab"},
	{"S-tab/h/l", "switch tab"},
	{"1-9", "seat"},
	{"v", "verdict"},
	{"↑↓/wheel", "scroll"},
	{"c", "copy"},
	{"f", "agent pane"},
	{"j", "judge now"},
	{"n", "new question"},
	{"q/esc", "close"},
	{"ctrl+c", "quit"},
	{"mouse", "tabs/buttons"},
}

func paddedCell(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func helpOverlay(base screen, width, height int) screen {
	col := min(33, (width-11)/2)
	panelWidth := 2*col + 7
	var panel screen
	panel.add(seg(dim.Render("+" + strings.Repeat("-", panelWidth-2) + "+")))
	title := bold.Render("COUNCIL / HELP")
	closeButton := button.Render("Close")
	panel.add(seg("| "+title+strings.Repeat(" ", panelWidth-4-lipgloss.Width(title)-lipgloss.Width(closeButton))), act(closeButton, "help-close"), seg(" |"))
	panel.add(seg("| " + paddedCell(accent.Render("ASK"), col) + " | " + paddedCell(accent.Render("WATCH"), col) + " |"))
	for i := 0; i < max(len(askHelp), len(watchHelp)); i++ {
		left, right := "", ""
		if i < len(askHelp) {
			left = keycap.Render(askHelp[i].key) + " " + dim.Render(askHelp[i].meaning)
		}
		if i < len(watchHelp) {
			right = keycap.Render(watchHelp[i].key) + " " + dim.Render(watchHelp[i].meaning)
		}
		panel.add(seg("| " + paddedCell(left, col) + " | " + paddedCell(right, col) + " |"))
	}
	panel.add(seg(dim.Render("+" + strings.Repeat("-", panelWidth-2) + "+")))
	x, y := max(0, (width-panelWidth)/2), max(0, (height-len(panel.lines))/2)
	base.zones = nil // only the overlay's Close button is active
	for len(base.lines) < height {
		base.lines = append(base.lines, "")
	}
	for i, line := range panel.lines {
		if y+i >= len(base.lines) {
			break
		}
		base.lines[y+i] = strings.Repeat(" ", x) + line
	}
	for _, z := range panel.zones {
		base.zones = append(base.zones, zone{z.y + y, z.x0 + x, z.x1 + x, z.act})
	}
	return base
}
