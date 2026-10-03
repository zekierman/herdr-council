package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// shorten works in terminal cells, including wide Unicode characters.
func shorten(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		cw := lipgloss.Width(string(r))
		if used+cw > width-1 {
			break
		}
		b.WriteRune(r)
		used += cw
	}
	return b.String() + "…"
}

func verdictState(s *Seat) string {
	if s == nil || s.Sent.IsZero() {
		return "waiting for answers"
	}
	switch s.State {
	case Done:
		return "ready to read"
	case Working, Sending:
		return "judge is weighing answers"
	case Blocked:
		return "judge needs attention"
	default:
		return "judge could not finish"
	}
}

// tabRows wraps complete clickable tabs; each action spans only its visible label.
func (m model) tabRows(now time.Time, width int) [][][2]string {
	var rows [][][2]string
	row := [][2]string{seg(" ")}
	used := 1
	add := func(label, action string) {
		cellWidth := lipgloss.Width(label)
		gap := 0
		if used > 1 {
			gap = 2
		}
		if used > 1 && used+gap+cellWidth > width {
			rows = append(rows, row)
			row = [][2]string{seg(" ")}
			used = 1
			gap = 0
		}
		if gap > 0 {
			row = append(row, seg("  "))
			used += 2
		}
		row = append(row, act(label, action))
		used += cellWidth
	}
	for i, s := range m.run.Seats {
		name := shorten(s.Agent.Name, 12)
		label := fmt.Sprintf("%d %s ", i+1, name)
		if i == m.seat {
			label = accent.Render("[" + label + "]")
		} else {
			label = dim.Render("[" + label + "]")
		}
		add(label+badge(s, m.frame, now), fmt.Sprintf("seat:%d", i))
	}
	j := m.run.Judge
	vb := dim.Render("· waiting")
	if j != nil && !j.Sent.IsZero() {
		vb = badge(j, m.frame, now)
	}
	vlabel := dim.Render("[VERDICT] ")
	if m.onVerdict() {
		vlabel = accent.Render("[VERDICT] ")
	}
	add(vlabel+vb, fmt.Sprintf("seat:%d", len(m.run.Seats)))
	rows = append(rows, row)
	return rows
}
